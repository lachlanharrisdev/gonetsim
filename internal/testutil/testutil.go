package testutil

import (
	"io"
	"log/slog"
	"net"
	"testing"
)

func Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func FreeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().String()
}

func FreePort(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket: %v", err)
		}
		defer func() { _ = pc.Close() }()
		return pc.LocalAddr().String()
	}
	return FreeTCPAddr(t)
}
