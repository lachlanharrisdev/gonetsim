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
		n, err := conn.Read(buf)
		if n > 0 {
			env.Capture.Write("", buf[:n])
		}
		if err != nil {
			return readError(err)
		}
	}
}

func (SinkHandler) HandleUDP(_ context.Context, data []byte, _ net.Addr, env Env) ([]byte, error) {
	env.Capture.Write("", data)
	return nil, nil
}
