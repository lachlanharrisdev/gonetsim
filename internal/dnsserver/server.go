package dnsserver

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

type Server struct {
	conf   Config
	server *dns.Server
}

func New(conf Config) (*Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	h := &handler{
		sinkholeIPv4: conf.SinkholeIPv4,
		sinkholeIPv6: conf.SinkholeIPv6,
	}

	// catch-all
	mux := dns.NewServeMux()
	mux.HandleFunc(".", h.handle)

	srv := &dns.Server{
		Addr:    conf.Addr,
		Net:     conf.Net,
		Handler: mux,
	}

	return &Server{conf: conf, server: srv}, nil
}

func (s *Server) ListenAndServe() error {
	log.Printf("dns: listening on %s (%s), sinkhole=%s", s.conf.Addr, s.conf.Net, s.sinkholeSummary())
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	_ = ctx // miekg/dns shutdown isn't ctx aware
	return s.server.Shutdown()
}

func (s *Server) sinkholeSummary() string {
	parts := []string{s.conf.SinkholeIPv4.String()}
	if s.conf.SinkholeIPv6.IsValid() {
		parts = append(parts, s.conf.SinkholeIPv6.String())
	}
	return strings.Join(parts, ",")
}

type handler struct {
	sinkholeIPv4 netip.Addr
	sinkholeIPv6 netip.Addr
}

func (h *handler) handle(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	for _, q := range r.Question {
		log.Printf("dns: query name=%s type=%s from=%s", q.Name, dns.TypeToString[q.Qtype], w.RemoteAddr())
		switch q.Qtype {
		case dns.TypeA:
			if rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", q.Name, h.sinkholeIPv4.String())); err == nil {
				m.Answer = append(m.Answer, rr)
			}
		case dns.TypeAAAA:
			if h.sinkholeIPv6.IsValid() {
				if rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN AAAA %s", q.Name, h.sinkholeIPv6.String())); err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		default:
			// ret NOERROR with empty Answer for other types
		}
	}

	_ = w.WriteMsg(m)
}
