package peers

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/DanArmor/GoTorrent/pkg/utils"
)

type Peer struct {
	IP   net.IP
	Port uint16
	PeerID [utils.PeerIDLen]byte
}

func Unmarshal(peersBin []byte, isIpv4 bool) ([]Peer, error) {
	peerSize := 18
	if isIpv4 {
		peerSize = 6
	}
	if len(peersBin)%peerSize != 0 {
		return nil, fmt.Errorf("malformed peers")
	}
	n := len(peersBin) / peerSize
	peers := make([]Peer, n)
	for i := 0; i < n; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+peerSize-2])
		peers[i].Port = binary.BigEndian.Uint16(peersBin[offset+peerSize-2 : offset+peerSize])
	}
	return peers, nil
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}
