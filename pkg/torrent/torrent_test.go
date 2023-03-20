package torrent

import (
	"encoding/hex"
	"testing"
)

func TestConstructTorrentV1(t *testing.T) {
	torrent, err := Parse("testdata/torrent_testdata1")
	if err != nil {
		t.Fatal(err)
	}

	infoHashAnswer := "6d4795dee70aeb88e03e5336ca7c9fcf0a1e206d"
	if hex.EncodeToString(torrent.InfoHash[:]) != infoHashAnswer {
		t.Errorf("wrong infohash:\nGot <%s>\nExp <%s>", hex.EncodeToString(torrent.InfoHash[:]), infoHashAnswer)
	}

	pieceLengthAnswer := 262144
	if torrent.PieceLength != pieceLengthAnswer {
		t.Errorf("wrong piece length:\nGot <%d>\nExp <%d>", torrent.PieceLength, pieceLengthAnswer)
	}

	lengthAnswer := 406847488
	if torrent.Length != lengthAnswer {
		t.Errorf("wrong length:\nGot <%d>\nExp <%d>", torrent.Length, lengthAnswer)
	}
}
