package dnsserver

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

func NewServer(conf Config) (*dns.Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	h := &handler{
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
	srv, err := NewServer(s.conf)
	if err != nil {
		return err
	}
	s.srv = srv

	log.Printf("dns: listening on %s (%s), sinkhole=%s", s.conf.Addr, s.conf.Net, sinkholeSummary(s.conf))
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
	sinkholeIPv4   netip.Addr
	sinkholeIPv6   netip.Addr
	sinkholeDomain string
	sinkholeTXT    string
	ttl            uint32
	compress       bool
}

func (h *handler) handle(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = h.compress

	for _, q := range r.Question {
		log.Printf("dns: query name=%s type=%s from=%s", q.Name, dns.TypeToString[q.Qtype], w.RemoteAddr())
		switch q.Qtype {
		case dns.TypeA:
			appendRecord(m, q, h.ttl, "A", h.sinkholeIPv4.String())
		case dns.TypeAAAA:
			if h.sinkholeIPv6.IsValid() {
				appendRecord(m, q, h.ttl, "AAAA", h.sinkholeIPv6.String())
			}
		case dns.TypeCNAME:
			appendRecord(m, q, h.ttl, "CNAME", h.sinkholeDomain)
		case dns.TypeMX:
			appendRecord(m, q, h.ttl, "MX", "10 "+h.sinkholeDomain)
		case dns.TypeTXT:
			appendRecord(m, q, h.ttl, "TXT", strconv.Quote(h.sinkholeTXT))
		case dns.TypeNS:
			appendRecord(m, q, h.ttl, "NS", h.sinkholeDomain)
		case dns.TypeSRV:
			appendRecord(m, q, h.ttl, "SRV", "10 0 0 "+h.sinkholeDomain)
		case dns.TypePTR:
			appendRecord(m, q, h.ttl, "PTR", h.sinkholeDomain)
		default:
			// ret NOERROR with empty Answer for other types
		}
	}

	_ = w.WriteMsg(m)
}

func appendRecord(m *dns.Msg, q dns.Question, ttl uint32, rrType, data string) {
	record := fmt.Sprintf("%s %d IN %s %s", q.Name, ttl, rrType, data)
	if rr, err := dns.NewRR(record); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create %s record for %s: %v", rrType, q.Name, err)
	}
}
