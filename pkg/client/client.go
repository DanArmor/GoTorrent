package client

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/handshake"
	"github.com/DanArmor/GoTorrent/pkg/msg"
	"github.com/DanArmor/GoTorrent/pkg/peers"
	"github.com/DanArmor/GoTorrent/pkg/utils"
)

type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield bitfield.Bitfield
	peer     peers.Peer
	infoHash [utils.InfoHashLen]byte
	peerID   [utils.PeerIDLen]byte
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
	bf, err := recvBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: bf,
		peer:     peer,
		infoHash: infoHash,
		peerID:   peerID,
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

func recvBitfield(conn net.Conn) (bitfield.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

	message, err := msg.Read(conn)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, fmt.Errorf("expected bitfield, but got nil")
	}
	if message.ID != msg.MsgBitfield {
		return nil, fmt.Errorf("expected bitfield but got ID %d", message.ID)
	}

	return message.Payload, nil
}

func (c *Client)Read(*msg.Message, error) (*msg.Message, error) {
	message, err := msg.Read(c.Conn)
	return message, err
}

func (c *Client) SendRequest(index int, begin int, length int) error {
	req := msg.FormatRequest(index, begin, length)
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendInterested() error {
	req := msg.Message{ID: msg.MsgInterested}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendNotInterested() error {
	req := msg.Message{ID: msg.MsgNotInterested}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendUnchoke() error {
	req := msg.Message{ID: msg.MsgUnchoke}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendHave(index int) error {
	req := msg.FormatHave(index)
	_, err := c.Conn.Write(req.Serialize())
	return err
}