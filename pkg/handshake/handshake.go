package handshake

import (
	"fmt"
	"io"

	"github.com/DanArmor/GoTorrent/pkg/utils"
)

type Handshake struct {
	Pstr     string
	InfoHash [utils.InfoHashLen]byte
	PeerID   [utils.PeerIDLen]byte
}

func (h *Handshake) Serialize() []byte {
	var reserved [8]byte
	buf := make([]byte, utils.HandshakeSize)
	buf[0] = byte(len(h.Pstr))
	curr := 1
	curr += copy(buf[curr:], []byte(h.Pstr))
	curr += copy(buf[curr:], reserved[:])
	curr += copy(buf[curr:], h.InfoHash[:])
	curr += copy(buf[curr:], h.PeerID[:])
	return buf
}

func Read(r io.Reader) (*Handshake, error) {
	buf := make([]byte, utils.HandshakeSize)
	n, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != utils.HandshakeSize {
		return nil, fmt.Errorf("wrong handshake size <%d>", n)
	}
	var (
		infoHash [utils.InfoHashLen]byte
		peerID   [utils.PeerIDLen]byte
	)
	copy(infoHash[:], buf[utils.ProtocolIDLen+1+8:utils.PeerIDLen+1+8+utils.InfoHashLen])
	copy(peerID[:], buf[utils.PeerIDLen+1+8+utils.InfoHashLen:])

	h := Handshake{
		Pstr:     string(buf[1:utils.ProtocolIDLen]),
		InfoHash: infoHash,
		PeerID:   peerID,
	}

	return &h, nil
}

func New(infoHash [utils.InfoHashLen]byte, peerID [utils.PeerIDLen]byte) *Handshake {
	return &Handshake{
		Pstr:     utils.ProtocolID,
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}
