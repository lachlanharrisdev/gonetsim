package dnsserver

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type RunOptions struct {
	ShutdownTimeout time.Duration
}

func NewServer(conf Config) (*dns.Server, error) {
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
	return srv, nil
}

func Run(ctx context.Context, conf Config, opts RunOptions) error {
	srv, err := NewServer(conf)
	if err != nil {
		return err
	}

	log.Printf("dns: listening on %s (%s), sinkhole=%s", conf.Addr, conf.Net, sinkholeSummary(conf))

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownTimeout := opts.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = 5 * time.Second
		}

		shutdownErr := make(chan error, 1)
		go func() { shutdownErr <- srv.Shutdown() }()

		select {
		case <-errCh:
			return nil
		case <-shutdownErr:
			return nil
		case <-time.After(shutdownTimeout):
			return nil
		}
	case err := <-errCh:
		return err
	}
}

func sinkholeSummary(conf Config) string {
	parts := []string{conf.SinkholeIPv4.String()}
	if conf.SinkholeIPv6.IsValid() {
		parts = append(parts, conf.SinkholeIPv6.String())
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
