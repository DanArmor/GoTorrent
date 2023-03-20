package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/p2p"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/utils"
	"github.com/jackpal/bencode-go"
)

const Port uint16 = 6881

type bencodeTorrent interface {
	toTorrentFile() (TorrentFile, error)
}

type bencodeInfoV1 struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeTrackerRespCompact struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

type bencodeTorrentV1 struct {
	Announce string        `bencode:"announce"`
	Info     bencodeInfoV1 `bencode:"info"`
}

func (bi *bencodeInfoV1) hash() ([utils.InfoHashLen]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *bi)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

func (bi *bencodeInfoV1) splitPieceHashes() ([][utils.PieceHashLen]byte, error) {
	buf := []byte(bi.Pieces)
	if len(buf)%utils.PieceHashLen != 0 {
		return nil, fmt.Errorf("malformed pieces of length %d", len(buf))
	}
	n := len(buf) / utils.PieceHashLen
	hashes := make([][utils.PieceHashLen]byte, n)
	for i := 0; i < n; i++ {
		copy(hashes[i][:], buf[i*utils.PieceHashLen:(i+1)*utils.PieceHashLen])
	}
	return hashes, nil
}

func (bt *bencodeTorrentV1) toTorrentFile() (TorrentFile, error) {
	infoHash, err := bt.Info.hash()
	if err != nil {
		return TorrentFile{}, nil
	}
	pieceHashes, err := bt.Info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, nil
	}
	t := TorrentFile{
		Announce:    bt.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bt.Info.PieceLength,
		Length:      bt.Info.Length,
		Name:        bt.Info.Name,
	}
	return t, nil
}

type TorrentFile struct {
	Announce    string
	InfoHash    [utils.InfoHashLen]byte
	PieceHashes [][utils.PieceHashLen]byte
	PieceLength int
	Length      int
	Name        string
}

func Parse(path string) (TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return TorrentFile{}, err
	}
	defer file.Close()

	bt := bencodeTorrentV1{}
	err = bencode.Unmarshal(file, &bt)
	if err != nil {
		return TorrentFile{}, err
	}
	return bt.toTorrentFile()
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
	url, err := tf.buildTrackerURL(peerID, port)
	if err != nil {
		return nil, err
	}

	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)
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

func (tf *TorrentFile) DownloadToFile(path string) error {
	var peerID [utils.PeerIDLen]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}
	peers, err := tf.requestPeers(peerID, Port)
	if err != nil {
		return err
	}
	torrent := p2p.Torrent{
		Peers:       peers,
		PeerID:      peerID,
		InfoHash:    tf.InfoHash,
		PieceHashes: tf.PieceHashes,
		PieceLength: tf.PieceLength,
		Length:      tf.Length,
		Name:        tf.Name,
	}

	buf, err := torrent.Download()
	if err != nil {
		return err
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = outFile.Write(buf)
	if err != nil {
		return err
	}

	return nil
}
