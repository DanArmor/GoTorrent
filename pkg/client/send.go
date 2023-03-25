package client

import (
	"github.com/DanArmor/GoTorrent/pkg/message"
)

func (c *Client) SendRequest(index int, begin int, length int) error {
	req := message.FormatRequest(index, begin, length)
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendInterested() error {
	req := message.Message{ID: message.MsgInterested}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendNotInterested() error {
	req := message.Message{ID: message.MsgNotInterested}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendUnchoke() error {
	req := message.Message{ID: message.MsgUnchoke}
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) SendHave(index int) error {
	req := message.FormatHave(index)
	_, err := c.Conn.Write(req.Serialize())
	return err
}
