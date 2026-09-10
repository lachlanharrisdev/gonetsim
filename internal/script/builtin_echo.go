package script

import (
	"context"
	"net"
)

// EchoHandler is the trivial smoke-test builtin: everything it reads is echoed
// back (TCP) or returned as the reply (UDP).
type EchoHandler struct{}

func (EchoHandler) ServeTCP(_ context.Context, conn net.Conn, _ Env) error {
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

func (EchoHandler) ServeUDP(_ context.Context, data []byte, _ net.Addr, _ Env) ([]byte, error) {
	return data, nil
}
