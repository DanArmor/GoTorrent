package torrentmeta

import (
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/p2p"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"github.com/DanArmor/GoTorrent/pkg/utils"
	"github.com/jackpal/bencode-go"
)

const Port uint16 = 6881

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
	Count      chan struct{}
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
	tfm.Count = make(chan struct{})
	return tfm
}

func (tf *TorrentFile) Save(path string) {
	f, err := os.Open(path)
	if err != nil {
		f, err = os.Create(path)
		if err != nil {
			panic(err)
		}
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
	tf.Count = make(chan struct{})
	f.Close()
}

func (tf *TorrentFile) buildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(tf.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(tf.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(tf.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

func (tf *TorrentFile) requestPeers(peerID [utils.PeerIDLen]byte, port uint16) ([]peers.Peer, error) {
	trackerUrl, err := tf.buildTrackerURL(peerID, port)
	if err != nil {
		return nil, err
	}
	
	p2p.WriteToLog(url.QueryEscape(string(tf.InfoHash[:])))
	p2p.WriteToLog(url.QueryEscape(string(peerID[:])))
	p2p.WriteToLog(trackerUrl)

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
		f, err := os.Open(tf.Files[i].FullPath)
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
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = torrent.Download(tf.Done, tf.Count)
	}()

	for range tf.Count {
		tf.Downloaded++
	}
	wg.Wait()
	tf.Out <- struct{}{}
	return nil
}
