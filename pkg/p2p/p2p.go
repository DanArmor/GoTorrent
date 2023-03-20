package p2p

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/client"
	"github.com/DanArmor/GoTorrent/pkg/msg"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/utils"
)

const MaxBlockSize = 16384

const MaxBacklog = 5

type Torrent struct {
	Peers       []peers.Peer
	PeerID      [utils.PeerIDLen]byte
	InfoHash    [utils.InfoHashLen]byte
	PieceHashes [][utils.PieceHashLen]byte
	PieceLength int
	Length      int
	Name        string
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
	message, err := state.client.Read()
	if err != nil {
		return err
	}

	if message == nil {
		return nil
	}

	switch message.ID {
	case msg.MsgUnchoke:
		state.client.Choked = false
	case msg.MsgChoke:
		state.client.Choked = true
	case msg.MsgHave:
		index, err := msg.ParseHave(message)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case msg.MsgPiece:
		n, err := msg.ParsePiece(state.index, state.buf, message)
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
	if end > t.Length {
		end = t.Length
	}
	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

func (t *Torrent) Download() ([]byte, error) {
	log.Printf("Starting downloading <%s>", t.Name)
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult, len(t.PieceHashes)/4)
	for index, hash := range t.PieceHashes {
		length := t.calculatePieceSize(index)
		workQueue <- &pieceWork{index, hash, length}
	}
	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}
	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHashes) {
		res := <-results
		begin, end := t.calculateBoundsForPiece(res.index)
		copy(buf[begin:end], res.buf)
		donePieces++
		percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
		numWorkers := runtime.NumGoroutine() - 1
		log.Printf("(%0.2f%%) Downloaded piece %d from %d peers", percent, res.index, numWorkers)
	}
	close(workQueue)
	return buf, nil
}
