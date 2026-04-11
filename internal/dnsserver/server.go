package dnsserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

func NewServer(conf Config, logger *slog.Logger) (*dns.Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

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

	srv := &dns.Server{
		Addr:    conf.Addr,
		Net:     conf.Net,
		Handler: mux,
	}
	return srv, nil
}

func (s *Server) Start(ctx context.Context) error {
	logger := s.log

	srv, err := NewServer(s.conf, logger)
	if err != nil {
		return err
	}
	s.srv = srv

	go func() {
		<-ctx.Done()
		err = s.srv.ShutdownContext(context.Background())
		if err != nil {
			logger.Error("failed to shutdown DNS server", "err", err)
		}
	}()

	logger.Info("listening", "on", s.conf.Addr, "net", s.conf.Net, "sinkhole", sinkholeSummary(s.conf))
	if err := srv.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.ShutdownContext(ctx)
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
