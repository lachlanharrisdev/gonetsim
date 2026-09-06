package dnsserver

import (
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/netx"
)

const AutoIPv4 = "auto"

func AutoSinkholeIPv4() netip.Addr {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			// AddrFromSlice returns ipv4 addresses as 4-in-6, unmap for Is4
			addr, ok := netip.AddrFromSlice(ipnet.IP)
			if !ok {
				continue
			}
			addr = addr.Unmap()
			if addr.Is4() && !addr.IsLoopback() && addr.IsGlobalUnicast() {
				return addr
			}
		}
	}
	return netip.MustParseAddr("127.0.0.1")
}

type Config struct {
	Addr string
	Net  string

	SinkholeIPv4   netip.Addr
	SinkholeIPv6   netip.Addr
	SinkholeDomain string
	SinkholeTXT    string
	TTL            uint32
	Compress       bool
	Capture        bool
}

// how long to keep a UDP capture writer open after its lastdatagram
const flowIdle = 5 * time.Minute

func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("listen addr is required")
	}
	if err := netx.ValidateNetwork(c.Net, "udp", "tcp", "both"); err != nil {
		return err
	}
	if !c.SinkholeIPv4.IsValid() {
		return errors.New("sinkhole ipv4 is required")
	}
	if c.SinkholeDomain == "" {
		return errors.New("sinkhole domain is required")
	}
	if c.SinkholeTXT == "" {
		return errors.New("sinkhole TXT is required")
	}

	return nil
}
