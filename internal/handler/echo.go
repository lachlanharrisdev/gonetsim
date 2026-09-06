package handler

import (
	"context"
	"net"
)

type EchoHandler struct{}

func (EchoHandler) HandleTCP(_ context.Context, conn net.Conn, env Env) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			if _, werr := conn.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return readError(err)
		}
	}
}

func (EchoHandler) HandleUDP(_ context.Context, data []byte, _ net.Addr, env Env) ([]byte, error) {
	return data, nil
}
