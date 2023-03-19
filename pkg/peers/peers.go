package peers

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func Unmarshal(peersBin []byte) ([]Peer, error) {
	const peerSize = 6
	if len(peersBin) % peerSize != 0 {
		return nil, fmt.Errorf("malformed peers")
	}
	n := len(peersBin) / peerSize
	peers := make([]Peer, n)
	for i := 0; i < n; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset + 4])
		peers[i].Port = binary.BigEndian.Uint16(peersBin[offset + 4 : offset + 6])
	}
	return peers, nil
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}