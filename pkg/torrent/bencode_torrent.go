package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"github.com/jackpal/bencode-go"
	"github.com/DanArmor/GoTorrent/pkg/utils"
)

type bencodeInfoV1 struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
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