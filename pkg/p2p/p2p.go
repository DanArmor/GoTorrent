package p2p

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/client"
	"github.com/DanArmor/GoTorrent/pkg/message"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"github.com/DanArmor/GoTorrent/pkg/utils"
)

const MaxBlockSize = 16384

const MaxBacklog = 5

var LogStrings []string
var LogMU sync.RWMutex

func WriteToLog(str string) {
	LogMU.Lock()
	LogStrings = append(LogStrings, str)
	LogMU.Unlock()
}

func GetLogString() string {
	LogMU.RLock()
	str := strings.Join(LogStrings, "\n")
	LogMU.RUnlock()
	return str
}

type File struct {
	torrent.File
	Handler *os.File
}

type Torrent struct {
	Peers       []peers.Peer
	PeerID      [utils.PeerIDLen]byte
	InfoHash    [utils.InfoHashLen]byte
	PieceHashes [][utils.PieceHashLen]byte
	PieceLength int
	Name        string
	Length      int
	Files       []File
	Bitfield    bitfield.Bitfield
}

type pieceWork struct {
	index  int
	hash   [utils.PieceHashLen]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	if msg == nil {
		return nil
	}

	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

func attemptDownloadPiece(ctx context.Context, c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}
	c.Conn.SetDeadline(time.Now().Add(1 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})
	timeoutCounter := 5
	for state.downloaded < pw.length {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("stopped by context")
		default:
			if !state.client.Choked {
				for state.backlog < MaxBacklog && state.requested < pw.length {
					blockSize := MaxBlockSize
					if pw.length-state.requested < blockSize {
						blockSize = pw.length - state.requested
					}
					err := c.SendRequest(pw.index, state.requested, blockSize)
					if err != nil {
						if err, ok := err.(net.Error); ok && err.Timeout() && timeoutCounter != 0 {
							c.Conn.SetDeadline(time.Now().Add(1 * time.Second))
							timeoutCounter--
							continue
						}
						return nil, err
					}
					state.backlog++
					state.requested += blockSize
				}
			}
			err := state.readMessage()
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() && timeoutCounter != 0 {
					c.Conn.SetDeadline(time.Now().Add(1 * time.Second))
					timeoutCounter--
					continue
				}
				return nil, err
			}
		}
	}

	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.hash[:]) {
		return fmt.Errorf("index %d failed integrity check", pw.index)
	}
	return nil
}

func (t *Torrent) startDownloadWorker(ctx context.Context, peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		WriteToLog(fmt.Sprintf("Could not handshake with %s. Disconnected", peer.IP))
		return
	}
	defer c.Conn.Close()
	WriteToLog(fmt.Sprintf("Completed handshake with %s", peer.IP))

	c.SendUnchoke()
	c.SendInterested()

	for {
		select {
		case <-ctx.Done():
			return
		case pw := <-workQueue:
			if !c.Bitfield.HasPiece(pw.index) {
				workQueue <- pw
				continue
			}
			buf, err := attemptDownloadPiece(ctx, c, pw)
			if err != nil {
				WriteToLog(fmt.Sprint("Exiting: ", err))
				workQueue <- pw
				return
			}
			err = checkIntegrity(pw, buf)
			if err != nil {
				WriteToLog(fmt.Sprintf("Piece %d failed integrity check", pw.index))
				workQueue <- pw
				continue
			}
			c.SendHave(pw.index)
			results <- &pieceResult{index: pw.index, buf: buf}
		}
	}
}

func (t *Torrent) calculateBoundsForPiece(index int) (begin int, end int) {
	begin = index * t.PieceLength
	end = begin + t.PieceLength
	if end > t.Files[len(t.Files)-1].End {
		end = t.Files[len(t.Files)-1].End
	}
	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

func (t *Torrent) writeToFile(pr pieceResult) {
	begin, _ := t.calculateBoundsForPiece(pr.index)
	index := -1
	for i := range t.Files {
		if begin <= t.Files[i].Begin {
			index = i-1
			break
		}
	}
	if index == -1{
		index = len(t.Files)-1
	}
	pieceLength := t.calculatePieceSize(pr.index)
	wrote := 0
	for i := index; i < len(t.Files); i++ {
		startInFile := 0
		if wrote == 0 {
			startInFile = begin - t.Files[i].Begin
		}
		endInFile := startInFile + pieceLength
		if endInFile > t.Files[i].Length {
			endInFile = t.Files[i].Length
		}
		//fmt.Print(fmt.Sprintf("i:%d wrote: %d, startInFile: %d  endInFile%d  len(pr.buf):%d  pieceLenght %d\n", pr.index, wrote, startInFile, endInFile, len(pr.buf), pieceLength))
		t.Files[i].Handler.WriteAt(pr.buf[wrote:wrote+endInFile-startInFile], int64(startInFile))
		wrote += endInFile - startInFile
		pieceLength -= endInFile - startInFile
		//WriteToLog(fmt.Sprintf("---wrote: %d, startInFile: %d  endInFile%d  len(pr.buf):%d  pieceLenght %d", wrote, startInFile, endInFile, len(pr.buf), pieceLength))
		if pieceLength == 0 {
			break
		}
	}
}

func (t *Torrent) Download(done chan struct{}, count chan int) {
	WriteToLog(fmt.Sprintf("Starting downloading <%s>", t.Name))
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult, len(t.PieceHashes)/4)
	donePieces := len(t.PieceHashes)
	for index, hash := range t.PieceHashes {
		if !t.Bitfield.HasPiece(index) {
			length := t.calculatePieceSize(index)
			workQueue <- &pieceWork{index, hash, length}
			donePieces--
		}
	}
	if donePieces == len(t.PieceHashes) {
		return
	}

	var wg sync.WaitGroup

	WriteToLog(fmt.Sprintf("Peers: %d", len(t.Peers)))
	ctx, cancel := context.WithCancel(context.Background())
	for _, peer := range t.Peers {
		wg.Add(1)
		go func(p peers.Peer) {
			defer wg.Done()
			t.startDownloadWorker(ctx, p, workQueue, results)
		}(peer)
	}

out:
	for donePieces < len(t.PieceHashes) {
		select {
		case <-done:
			cancel()
			break out
		case res := <-results:
			t.writeToFile(*res)
			donePieces++
			count <- res.index
			t.Bitfield.SetPiece(res.index)
		}
	}
	cancel()
	for i := range t.Files {
		t.Files[i].Handler.Close()
	}
	wg.Wait()
	close(workQueue)
	close(count)
	close(results)
}
