package dnsserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

func NewServers(conf Config, logger *slog.Logger) ([]*dns.Server, error) {
	h := &handler{
		logger:         logger,
		sinkholeIPv4:   conf.SinkholeIPv4,
		sinkholeIPv6:   conf.SinkholeIPv6,
		sinkholeDomain: conf.SinkholeDomain,
		sinkholeTXT:    conf.SinkholeTXT,
		ttl:            conf.TTL,
		compress:       conf.Compress,
	}

	// catch-all
	mux := dns.NewServeMux()
	mux.HandleFunc(".", h.handle)

	network := strings.ToLower(strings.TrimSpace(conf.Net))
	switch network {
	case "udp", "tcp":
		return []*dns.Server{{Addr: conf.Addr, Net: network, Handler: mux}}, nil
	case "both":
		return []*dns.Server{
			{Addr: conf.Addr, Net: "udp", Handler: mux},
			{Addr: conf.Addr, Net: "tcp", Handler: mux},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported dns network %q (must be udp, tcp, or both)", conf.Net)
	}
}

func NewServer(conf Config, logger *slog.Logger) (*dns.Server, error) {
	srvs, err := NewServers(conf, logger)
	if err != nil {
		return nil, err
	}
	if len(srvs) != 1 {
		return nil, fmt.Errorf("expected 1 dns server, got %d", len(srvs))
	}
	return srvs[0], nil
}

func (s *Server) Start(ctx context.Context) error {
	logger := s.log

	srvs, err := NewServers(s.conf, logger)
	if err != nil {
		return err
	}
	s.srvs = srvs

	netLabel := strings.ToLower(strings.TrimSpace(s.conf.Net))
	if netLabel == "both" {
		netLabel = "udp+tcp"
	}

	logger.Info("listening", "on", s.conf.Addr, "net", netLabel, "sinkhole", sinkholeSummary(s.conf))

	errCh := make(chan error, len(srvs))
	for _, srv := range srvs {
		srv := srv
		go func() {
			errCh <- srv.ListenAndServe()
		}()
	}

	var retErr error
	for i := 0; i < len(srvs); i++ {
		err := <-errCh
		if err != nil && retErr == nil {
			retErr = err
			for _, srv := range srvs {
				_ = srv.Shutdown()
			}
		}
	}
	return retErr
}

func (s *Server) Stop(ctx context.Context) error {
	if len(s.srvs) == 0 {
		return nil
	}

	var firstErr error
	for _, srv := range s.srvs {
		if err := srv.ShutdownContext(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.srvs = nil
	return firstErr
}

func sinkholeSummary(conf Config) string {
	parts := []string{conf.SinkholeIPv4.String()}
	if conf.SinkholeIPv6.IsValid() {
		parts = append(parts, conf.SinkholeIPv6.String())
	}
	return strings.Join(parts, ",")
}

type handler struct {
	logger *slog.Logger

	sinkholeIPv4   netip.Addr
	sinkholeIPv6   netip.Addr
	sinkholeDomain string
	sinkholeTXT    string
	ttl            uint32
	compress       bool
}

func (h *handler) handle(w dns.ResponseWriter, r *dns.Msg) {
	logger := h.logger

	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = h.compress

	for _, q := range r.Question {
		qtype := dns.TypeToString[q.Qtype]
		logger.Info(qtype, "src", w.RemoteAddr().String(), "to", strings.TrimSuffix(q.Name, "."))
		switch q.Qtype {
		case dns.TypeA:
			appendRecord(logger, m, q, h.ttl, "A", h.sinkholeIPv4.String())
		case dns.TypeAAAA:
			if h.sinkholeIPv6.IsValid() {
				appendRecord(logger, m, q, h.ttl, "AAAA", h.sinkholeIPv6.String())
			}
		case dns.TypeCNAME:
			appendRecord(logger, m, q, h.ttl, "CNAME", h.sinkholeDomain)
		case dns.TypeMX:
			appendRecord(logger, m, q, h.ttl, "MX", "10 "+h.sinkholeDomain)
		case dns.TypeTXT:
			appendRecord(logger, m, q, h.ttl, "TXT", h.sinkholeTXT)
		case dns.TypeNS:
			appendRecord(logger, m, q, h.ttl, "NS", h.sinkholeDomain)
		case dns.TypeSRV:
			appendRecord(logger, m, q, h.ttl, "SRV", "10 0 0 "+h.sinkholeDomain)
		case dns.TypePTR:
			appendRecord(logger, m, q, h.ttl, "PTR", h.sinkholeDomain)
		case dns.TypeSOA:
			appendRecord(logger, m, q, h.ttl, "SOA", fmt.Sprintf("%s. hostmaster.%s. 1 3600 600 604800 3600", h.sinkholeDomain, h.sinkholeDomain))
		case dns.TypeCAA:
			appendRecord(logger, m, q, h.ttl, "CAA", fmt.Sprintf("0 issue \"%s\"", h.sinkholeDomain))
		default:
			// ret NOERROR with empty Answer for other types
		}
	}

	_ = w.WriteMsg(m)
}

func appendRecord(logger *slog.Logger, m *dns.Msg, q dns.Question, ttl uint32, rrType, data string) {
	record := fmt.Sprintf("%s %d IN %s %s", q.Name, ttl, rrType, data)
	if rr, err := dns.NewRR(record); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		logger.Error("failed to create record", "type", rrType, "name", q.Name, "err", err)
	}
}
