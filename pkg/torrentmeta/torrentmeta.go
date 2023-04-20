package torrentmeta

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/p2p"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"github.com/DanArmor/GoTorrent/pkg/utils"
	"github.com/jackpal/bencode-go"
)

const Port uint16 = 36010

type bencodeTrackerRespCompact struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

type TorrentFile struct {
	torrent.TorrentFile
	Bitfield   bitfield.Bitfield
	Downloaded int
	Uploaded   int
	Done       chan struct{}
	Count      chan int
	Out        chan struct{}
	InProgress bool
	IsDone     bool
}

func New(path string, downloadPath string) TorrentFile {
	tf, err := torrent.Parse(path)
	if err != nil {
		panic(err)
	}
	tfm := TorrentFile{
		TorrentFile: tf,
	}
	tfm.Bitfield = make(bitfield.Bitfield, len(tfm.PieceHashes)/8+1)
	for i := range tfm.Files {
		tfm.Files[i].FullPath = filepath.Join(downloadPath, tfm.Files[i].FullPath)
	}
	tfm.Done = make(chan struct{})
	tfm.Out = make(chan struct{})
	tfm.Count = make(chan int)
	return tfm
}

func (tf *TorrentFile) Save(path string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	err = gob.NewEncoder(f).Encode(tf)
	if err != nil {
		panic(err)
	}
	f.Close()
}

func (tf *TorrentFile) Load(path string) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	err = gob.NewDecoder(f).Decode(tf)
	if err != nil {
		panic(err)
	}
	tf.Done = make(chan struct{})
	tf.Out = make(chan struct{})
	tf.Count = make(chan int)
	f.Close()
}

func (tf *TorrentFile) BuildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(tf.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(tf.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{strconv.Itoa(tf.Uploaded)},
		"downloaded": []string{strconv.Itoa(tf.Downloaded)},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(len(tf.PieceHashes) - tf.Downloaded)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

func (tf *TorrentFile) requestPeers(peerID [utils.PeerIDLen]byte, port uint16) ([]peers.Peer, error) {
	trackerUrl, err := tf.BuildTrackerURL(peerID, port)
	if err != nil {
		return nil, err
	}
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(trackerUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	trackerResp := bencodeTrackerRespCompact{}
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	if err != nil {
		return nil, err
	}
	return peers.Unmarshal([]byte(trackerResp.Peers))
}

func (t *TorrentFile) CheckFilesIntegrity() bool {
	var err error
	fileIndex := 0
	handlers := make([]*os.File, len(t.Files))
	for i := range handlers {
		handlers[i], err = os.Open(t.Files[i].FullPath)
		if err != nil {
			panic(err)
		}
		defer func(index int) {
			handlers[index].Close()
		}(i)
	}
	buf := make([]byte, t.PieceLength)
	for i := range t.PieceHashes {
		n, err := handlers[fileIndex].Read(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				fileIndex++
			}
		}
		if n != t.PieceLength {
			for {
				if fileIndex == len(t.Files) {
					return t.CheckIntegrity(t.PieceHashes[i], buf)
				}
				r, err := handlers[fileIndex].Read(buf[n+1:])
				if err != nil {
					if err != io.EOF {
						panic(err)
					} else {
						fileIndex++
					}
				}
				n += r
				if n == t.PieceLength {
					break
				}
			}
		}
		if !t.CheckIntegrity(t.PieceHashes[i], buf) {
			return false
		}
	}
	return true
}

func (t *TorrentFile) CheckIntegrity(pw [utils.PieceHashLen]byte, buf []byte) bool {
	hash := sha1.Sum(buf)
	return bytes.Equal(hash[:], pw[:])
}

func (tf *TorrentFile) DownloadToFile() error {
	var peerID [utils.PeerIDLen]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}
	peers, err := tf.requestPeers(peerID, Port)
	p2p.WriteToLog(fmt.Sprint(peers))
	if err != nil {
		return err
	}

	var p2pFiles []p2p.File

	for i := range tf.Files {
		f, err := os.OpenFile(tf.Files[i].FullPath, os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}
		p2pFiles = append(p2pFiles, p2p.File{File: tf.Files[i], Handler: f})
	}

	torrent := p2p.Torrent{
		Peers:       peers,
		PeerID:      peerID,
		InfoHash:    tf.InfoHash,
		PieceHashes: tf.PieceHashes,
		PieceLength: tf.PieceLength,
		Files:       p2pFiles,
		Name:        tf.Name,
		Length:      tf.Length,
		Bitfield:    tf.Bitfield,
	}

	go func() {
		torrent.Download(tf.Done, tf.Count)
	}()

	for index := range tf.Count {
		tf.Downloaded++
		tf.Bitfield.SetPiece(index)
	}
	if tf.Downloaded != len(tf.PieceHashes) {
		tf.Out <- struct{}{}
	}
	return nil
}
