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
			aResponse(q, h.sinkholeIPv4, h.ttl, m)
		case dns.TypeAAAA:
			if h.sinkholeIPv6.IsValid() {
				aaaaResponse(q, h.sinkholeIPv6, h.ttl, m)
			}
		case dns.TypeCNAME:
			cnameResponse(q, h.sinkholeDomain, h.ttl, m)
		case dns.TypeMX:
			mxResponse(q, h.sinkholeDomain, h.ttl, m)
		case dns.TypeTXT:
			txtResponse(q, h.sinkholeTXT, h.ttl, m)
		case dns.TypeNS:
			nsResponse(q, h.sinkholeDomain, h.ttl, m)
		case dns.TypeSRV:
			srvResponse(q, h.sinkholeDomain, h.ttl, m)
		case dns.TypePTR:
			ptrResponse(q, h.sinkholeDomain, h.ttl, m)
		default:
			// ret NOERROR with empty Answer for other types
		}
	}

	_ = w.WriteMsg(m)
}

func aResponse(q dns.Question, ip netip.Addr, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN A %s", q.Name, ttl, ip.String())); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create A record for %s: %v", q.Name, err)
	}
}

func aaaaResponse(q dns.Question, ip netip.Addr, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN AAAA %s", q.Name, ttl, ip.String())); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create AAAA record for %s: %v", q.Name, err)
	}
}

func cnameResponse(q dns.Question, target string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN CNAME %s", q.Name, ttl, target)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create CNAME record for %s: %v", q.Name, err)
	}
}

func mxResponse(q dns.Question, target string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN MX 10 %s", q.Name, ttl, target)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create MX record for %s: %v", q.Name, err)
	}
}

func txtResponse(q dns.Question, txt string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN TXT %q", q.Name, ttl, txt)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create TXT record for %s: %v", q.Name, err)
	}
}

func nsResponse(q dns.Question, target string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN NS %s", q.Name, ttl, target)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create NS record for %s: %v", q.Name, err)
	}
}

func srvResponse(q dns.Question, target string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN SRV 10 0 0 %s", q.Name, ttl, target)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create SRV record for %s: %v", q.Name, err)
	}
}

func ptrResponse(q dns.Question, target string, ttl uint32, m *dns.Msg) {
	if rr, err := dns.NewRR(fmt.Sprintf("%s %d IN PTR %s", q.Name, ttl, target)); err == nil {
		m.Answer = append(m.Answer, rr)
	} else {
		log.Printf("dns: failed to create PTR record for %s: %v", q.Name, err)
	}
}
