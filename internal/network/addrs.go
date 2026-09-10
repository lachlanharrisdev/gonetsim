package network

import (
	"errors"
	"fmt"
	"net"
)

var errListenAddrRequired = errors.New("listen address is required")

// ParseAddr validates that addr is a usable TCP/UDP host:port.
func ParseAddr(addr string) (string, error) {
	if addr == "" {
		return "", errListenAddrRequired
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return "", fmt.Errorf("invalid listen address %q (expected host:port): %w", addr, err)
	}
	return addr, nil
}
