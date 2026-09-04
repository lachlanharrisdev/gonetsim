package dnsserver

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"

	"github.com/miekg/dns"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
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

func (s *Server) Name() string {
	return "DNS"
}

type Server struct {
	conf Config
	srvs []*dns.Server
	log  *slog.Logger
}

func NewService(conf Config, logger *slog.Logger) service.Service {
	return &Server{conf: conf, log: service.NewPrefixedLogger(logger, "DNS")}
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
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("listen addr is required")
	}
	if c.Net == "" {
		return errors.New("network is required")
	}
	net := strings.ToLower(strings.TrimSpace(c.Net))
	switch net {
	case "udp", "tcp", "both":
		// all good my boy
	default:
		return errors.New("network must be one of: udp, tcp, both")
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
