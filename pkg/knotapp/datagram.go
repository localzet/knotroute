package knotapp

import (
	"context"
	"errors"
	"net"
)

const DefaultDatagramLimit int64 = 256 << 10

type DatagramClient struct {
	Dialer Dialer
	Limit  int64
}

func (c DatagramClient) Send(ctx context.Context, address string, payload []byte) error {
	if c.Dialer == nil {
		return errors.New("datagram dialer is nil")
	}
	limit := c.Limit
	if limit <= 0 {
		limit = DefaultDatagramLimit
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeBlob(conn, payload, limit)
}

func DatagramServer(limit int64, handler func([]byte)) func(net.Conn) {
	if limit <= 0 {
		limit = DefaultDatagramLimit
	}
	return func(conn net.Conn) {
		payload, err := readBlob(conn, limit)
		if err == nil && handler != nil {
			handler(payload)
		}
	}
}
