package cmd

import (
	"fmt"
	"net"
	"net/netip"
)

func parseAddrPort(listen string) (string, error) {
	if listen == "" {
		return "", fmt.Errorf("listen address is required")
	}

	if _, err := net.ResolveTCPAddr("tcp", listen); err != nil {
		return "", fmt.Errorf("invalid listen address %q (expected host:port): %w", listen, err)
	}

	return listen, nil
}

func parseNetipAddr(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid ip %q: %w", s, err)
	}
	return a, nil
}

func parseOptionalNetipAddr(s string) (netip.Addr, error) {
	if s == "" {
		return netip.Addr{}, nil
	}
	return parseNetipAddr(s)
}
