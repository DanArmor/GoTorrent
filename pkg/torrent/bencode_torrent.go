package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"github.com/DanArmor/GoTorrent/pkg/utils"
	"github.com/jackpal/bencode-go"
)

type bencodeInfoV1 struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length,omitempty"`
	Name        string `bencode:"name"`
	NameUtf8    string `bencode:"name.utf-8,omitempty"`
	Private     *bool  `bencode:"private,omitempty"`
	Source      string `bencode:"source,omitempty"`
	Files       []File `bencode:"files,omitempty"`
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

func (bt *bencodeTorrentV1) calculateFilesBounds() int {
	totalSize := 0
	for i := range bt.Info.Files {
		bt.Info.Files[i].Begin = totalSize
		totalSize += bt.Info.Files[i].Length
		bt.Info.Files[i].End = totalSize
	}
	return totalSize
}

func (bt *bencodeTorrentV1) calculateFullPaths() {
	for i := range bt.Info.Files {
		bt.Info.Files[i].FullPath = filepath.Join(bt.Info.Files[i].Path...)
	}
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
	isMultiple := bt.Info.Length == 0
	if !isMultiple {
		bt.Info.Files = append(bt.Info.Files, File{Length: bt.Info.Length, Path: []string{bt.Info.Name}})
	}
	totalSize := bt.calculateFilesBounds()
	bt.calculateFullPaths()
	t := TorrentFile{
		Announce:    bt.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bt.Info.PieceLength,
		Length:      bt.Info.Length,
		Name:        bt.Info.Name,
		Files:       bt.Info.Files,
		TotalSize:   totalSize,
		IsMultiple:  isMultiple,
	}
	if isMultiple {
		for i := range t.Files {
			t.Files[i].FullPath = filepath.Join(t.Name, t.Files[i].FullPath)
		}
	}
	return t, nil
}
