package netx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

func ParseAddr(addr string) (string, error) {
	if addr == "" {
		return "", fmt.Errorf("listen address is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return "", fmt.Errorf("invalid listen address %q (expected host:port): %w", addr, err)
	}
	return addr, nil
}

func ValidateNetwork(network string, allowed ...string) error {
	n := strings.ToLower(strings.TrimSpace(network))
	for _, a := range allowed {
		if n == a {
			return nil
		}
	}
	return fmt.Errorf("network must be one of: %s", strings.Join(allowed, ", "))
}

func ValidateStatus(code int) error {
	if code != 0 && (code < 100 || code > 599) {
		return fmt.Errorf("status code must be 0 or between 100 and 599, was %d", code)
	}
	return nil
}

func DisplayNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "both":
		return "udp+tcp"
	case "tcp":
		return "tcp"
	default:
		return "udp"
	}
}

func ParsePort(addr string) (int, bool) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, false
	}
	return port, true
}

func ListenTCP(addr string, run *capture.Run, iface int, tlsCfg *tls.Config) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	ln = capture.NewConnListener(ln, run, iface)
	if tlsCfg != nil {
		ln = tls.NewListener(ln, tlsCfg)
	}
	return ln, nil
}

func ListenUDP(addr string, run *capture.Run, iface int, idle time.Duration) (*capture.PacketConn, error) {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	return capture.NewPacketConn(pc, run, iface, idle), nil
}

func CloseOnCancel(ctx context.Context, c io.Closer) (stop func()) {
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-stopped:
		}
	}()
	return func() { close(stopped) }
}

func IsExpectedClose(err error, ctx context.Context) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	return ctx.Err() != nil
}
