package p2p

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"runtime"
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

type File struct {
	torrent.File
	handler *os.File
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

func attemptDownloadPiece(c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}
	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}
				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}
		err := state.readMessage()
		if err != nil {
			return nil, err
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

func (t *Torrent) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		log.Printf("Could not handshake with %s. Disconnected", peer.IP)
		return
	}
	defer c.Conn.Close()
	log.Printf("Completed handshake with %s", peer.IP)

	c.SendUnchoke()
	c.SendInterested()

	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}
		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			log.Print("Exiting", err)
			workQueue <- pw
			return
		}
		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("Piece %d failed integrity check", pw.index)
			workQueue <- pw
			continue
		}
		c.SendHave(pw.index)
		results <- &pieceResult{index: pw.index, buf: buf}
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
	index := 0
	for i := range t.Files {
		if begin >= t.Files[i].Begin {
			index = i
			break
		}
	}
	pieceLength := t.calculatePieceSize(pr.index)
	wrote := 0
	for i := index; i < len(t.Files); i++ {
		startInFile := begin - t.Files[i].Begin
		endInFile := startInFile + pieceLength
		if endInFile > t.Files[i].Length {
			endInFile = t.Files[i].Length
		}
		t.Files[i].handler.WriteAt(pr.buf[wrote:wrote+endInFile-startInFile], int64(startInFile))
		wrote += endInFile - startInFile
		pieceLength -= wrote
		if pieceLength == 0 {
			break
		}
	}
}

func (t *Torrent) Download(done chan struct{}) (int, error) {
	log.Printf("Starting downloading <%s>", t.Name)
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult, len(t.PieceHashes)/4)
	for index, hash := range t.PieceHashes {
		if !t.Bitfield.HasPiece(index){
			length := t.calculatePieceSize(index)
			workQueue <- &pieceWork{index, hash, length}
		}
	}
	
	var wg sync.WaitGroup

	for _, peer := range t.Peers {
		wg.Add(1)
		go func(p peers.Peer){
			defer wg.Done()
			t.startDownloadWorker(p, workQueue, results)
		}(peer)
	}
	donePieces := 0
	out:
	for donePieces < len(t.PieceHashes) {
		select {
		case <- done:
			break out
		default:
			res := <-results
			t.writeToFile(*res)
			donePieces++
			t.Bitfield.SetPiece(res.index)
			
			percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
			numWorkers := runtime.NumGoroutine() - 1
			log.Printf("(%0.2f%%) Downloaded piece %d from %d peers", percent, res.index, numWorkers)
		}

	}
	close(workQueue)
	wg.Wait()
	return donePieces, nil
}
