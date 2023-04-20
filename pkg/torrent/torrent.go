package torrent

import (
	"github.com/DanArmor/GoTorrent/pkg/utils"
	"github.com/jackpal/bencode-go"
	"os"
)

type File struct {
	Length   int      `bencode:"length"`
	Path     []string `bencode:"path"`
	PathUtf8 []string `bencode:"path.utf-8,omitempty"`
	FullPath string   `bencode:"-"`
	Begin    int      `bencode:"-"`
	End      int      `bencode:"-"`
}

type TorrentFile struct {
	Announce     string
	InfoHash     [utils.InfoHashLen]byte
	PieceHashes  [][utils.PieceHashLen]byte
	PieceLength  int
	Length       int
	Name         string
	Files        []File
	TotalSize    int
	IsMultiple   bool
	CreatedBy    string
	Comment      string
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

	tf, err := bt.toTorrentFile()
	if err != nil {
		return TorrentFile{}, err
	}
	return tf, nil
}
