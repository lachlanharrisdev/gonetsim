package dnsserver

import (
	"errors"
	"log/slog"
	"net/netip"

	"github.com/miekg/dns"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
)

func (s *Server) Name() string {
	return "DNS"
}

type Server struct {
	conf Config
	srv  *dns.Server
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

func (c Config) validate() error {
	if c.Addr == "" {
		return errors.New("dns listen addr is required")
	}
	if c.Net == "" {
		return errors.New("dns network is required")
	}
	if !c.SinkholeIPv4.IsValid() {
		return errors.New("dns sinkhole ipv4 is required")
	}
	if c.SinkholeDomain == "" {
		return errors.New("dns sinkhole domain is required")
	}
	if c.SinkholeTXT == "" {
		return errors.New("dns sinkhole TXT is required")
	}

	return nil
}
