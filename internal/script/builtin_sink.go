package script

import (
	"context"
	"net"
)

// SinkHandler consumes and discards all received data. Useful for draining a
// socket without generating noise.
type SinkHandler struct{}

func (SinkHandler) ServeTCP(_ context.Context, conn net.Conn, _ Env) error {
	buf := make([]byte, 32*1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			return readError(err)
		}
	}
}

func (SinkHandler) ServeUDP(_ context.Context, _ []byte, _ net.Addr, _ Env) ([]byte, error) {
	return nil, nil
}
