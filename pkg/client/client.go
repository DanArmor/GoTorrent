package client

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/handshake"
	"github.com/DanArmor/GoTorrent/pkg/message"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/utils"
)

type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield bitfield.Bitfield
	peer     peers.Peer
	InfoHash [utils.InfoHashLen]byte
	PeerID   [utils.PeerIDLen]byte
}

func CheckHandshake(peer peers.Peer, peerID [utils.PeerIDLen]byte, infoHash [utils.InfoHashLen]byte) error {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
	if err != nil {
		return err
	}
	_, err = completeHandshake(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return err
	}
	return nil
}

func New(peer peers.Peer, peerID [utils.PeerIDLen]byte, infoHash [utils.InfoHashLen]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = completeHandshake(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}
	bf, err := RecvBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: bf,
		peer:     peer,
		InfoHash: infoHash,
		PeerID:   peerID,
	}, nil
}

func completeHandshake(conn net.Conn, infoHash [utils.InfoHashLen]byte, peerID [utils.PeerIDLen]byte) (*handshake.Handshake, error) {
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	defer conn.SetDeadline(time.Time{})

	req := handshake.New(infoHash, peerID)
	_, err := conn.Write(req.Serialize())
	if err != nil {
		return nil, err
	}

	res, err := handshake.Read(conn)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(res.InfoHash[:], infoHash[:]) {
		return nil, fmt.Errorf("wrong infohash %s (got)/(expected) %s", res.InfoHash, infoHash)
	}
	return res, nil
}

func RecvBitfield(conn net.Conn) (bitfield.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

	msg, err := message.Read(conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, fmt.Errorf("expected bitfield, but got nil")
	}
	if msg.ID != message.MsgBitfield {
		return nil, fmt.Errorf("expected bitfield but got ID %d", msg.ID)
	}

	return msg.Payload, nil
}

func (c *Client) Read() (*message.Message, error) {
	msg, err := message.Read(c.Conn)
	return msg, err
}