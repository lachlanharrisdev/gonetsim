package handler

import (
	"context"
	"net"
)

// SinkHandler consumes and discards all received data.
type SinkHandler struct{}

func (SinkHandler) HandleTCP(_ context.Context, conn net.Conn, env Env) error {
	buf := make([]byte, 32*1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			return readError(err)
		}
	}
}

func (SinkHandler) HandleUDP(_ context.Context, data []byte, _ net.Addr, env Env) ([]byte, error) {
	return nil, nil
}
