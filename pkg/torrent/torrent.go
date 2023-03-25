package torrent

import (
	"crypto/rand"
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

type bencodeTrackerRespCompact struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
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
