package dnsserver

import (
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
)

// not a test in of itself; sets up config and server for all record-specific tests (e.g. A, AAAA, TXT) to use, to avoid duplication of setup code in each test
func queryTestsHelper(t *testing.T) (client *dns.Client, addr string, config Config, teardown func()) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		// failed to listen on a local udp port with error
		t.Fatalf("ListenPacket: %v", err)
	}
	addr = pc.LocalAddr().String()

	conf := Config{
		Addr:           addr,
		Net:            "udp",
		SinkholeIPv4:   netip.MustParseAddr("203.0.113.10"),
		SinkholeIPv6:   netip.MustParseAddr("2001:db8::10"),
		SinkholeDomain: "localhost",
		SinkholeTXT:    "test",
		TTL:            60,
		Compress:       false,
	}
	logger := testutil.Logger()
	srv, err := NewServer(conf, logger)
	if err != nil {
		// failed to create server with error
		_ = pc.Close()
		t.Fatalf("New: %v", err)
	}

	srv.PacketConn = pc

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ActivateAndServe()
	}()
	teardown = func() {
		err = srv.Shutdown()
		if err != nil {
			// failed to shutdown server with error
			t.Fatalf("Shutdown: %v", err)
		}
		select {
		case <-errCh:
		case <-time.After(500 * time.Millisecond):
		}
	}

	client = &dns.Client{Net: "udp", Timeout: 1 * time.Second}

	return client, addr, conf, teardown
}

func queryBothTransportsHelper(t *testing.T) (udpClient *dns.Client, tcpClient *dns.Client, addr string, config Config, teardown func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	pc, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		_ = ln.Close()
		t.Fatalf("ListenPacket: %v", err)
	}

	addr = fmt.Sprintf("127.0.0.1:%d", port)

	conf := Config{
		Addr:           addr,
		Net:            "both",
		SinkholeIPv4:   netip.MustParseAddr("203.0.113.10"),
		SinkholeIPv6:   netip.MustParseAddr("2001:db8::10"),
		SinkholeDomain: "localhost",
		SinkholeTXT:    "test",
		TTL:            60,
		Compress:       false,
	}
	logger := testutil.Logger()

	srvs, err := NewServers(conf, logger)
	if err != nil {
		_ = pc.Close()
		_ = ln.Close()
		t.Fatalf("NewServers: %v", err)
	}
	for _, srv := range srvs {
		switch srv.Net {
		case "udp":
			srv.PacketConn = pc
		case "tcp":
			srv.Listener = ln
		default:
			_ = pc.Close()
			_ = ln.Close()
			t.Fatalf("unexpected server net: %q", srv.Net)
		}
	}

	errCh := make(chan error, len(srvs))
	for _, srv := range srvs {
		srv := srv
		go func() {
			errCh <- srv.ActivateAndServe()
		}()
	}

	teardown = func() {
		for _, srv := range srvs {
			if err := srv.Shutdown(); err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		}
		_ = pc.Close()
		_ = ln.Close()

		for i := 0; i < len(srvs); i++ {
			select {
			case <-errCh:
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	udpClient = &dns.Client{Net: "udp", Timeout: 1 * time.Second}
	tcpClient = &dns.Client{Net: "tcp", Timeout: 1 * time.Second}

	return udpClient, tcpClient, addr, conf, teardown
}

func TestAutoSinkholeIPv4(t *testing.T) {
	addr := AutoSinkholeIPv4()
	if !addr.IsValid() || !addr.Is4() {
		t.Fatalf("expected a valid IPv4 address, got %v", addr)
	}
}

func TestRecordTypes(t *testing.T) {
	cases := []struct {
		name  string
		qname string
		qtype uint16
		check func(t *testing.T, resp *dns.Msg, conf Config)
	}{
		{"wildcard", "random-beacon-9f3a.malware.example.", dns.TypeA, checkA},
		{"A", "example.com.", dns.TypeA, checkA},
		{"AAAA", "example.com.", dns.TypeAAAA, checkAAAA},
		{"TXT", "example.com.", dns.TypeTXT, checkTXT},
		{"CNAME", "example.com.", dns.TypeCNAME, checkDomainTarget},
		{"MX", "example.com.", dns.TypeMX, checkDomainTarget},
		{"NS", "example.com.", dns.TypeNS, checkDomainTarget},
		{"SRV", "_sip._tcp.example.com.", dns.TypeSRV, checkDomainTarget},
		{"PTR", "example.com.", dns.TypePTR, checkDomainTarget},
		{"SOA", "example.com.", dns.TypeSOA, checkSOA},
		{"CAA", "example.com.", dns.TypeCAA, checkCAA},
	}
	client, addr, conf, teardown := queryTestsHelper(t)
	defer teardown()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := exchange(t, client, addr, tc.qname, tc.qtype)
			if len(resp.Answer) != 1 {
				t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
			}
			tc.check(t, resp, conf)
		})
	}
}

func checkA(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A, got %T", resp.Answer[0])
	}
	if got := a.A.String(); got != conf.SinkholeIPv4.String() {
		t.Fatalf("expected %s, got %s", conf.SinkholeIPv4.String(), got)
	}
}

func checkAAAA(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	aaaa, ok := resp.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("expected *dns.AAAA, got %T", resp.Answer[0])
	}
	if got := aaaa.AAAA.String(); got != conf.SinkholeIPv6.String() {
		t.Fatalf("expected %s, got %s", conf.SinkholeIPv6.String(), got)
	}
}

func checkTXT(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	txt, ok := resp.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("expected *dns.TXT, got %T", resp.Answer[0])
	}
	if len(txt.Txt) != 1 {
		t.Fatalf("expected 1 TXT record, got %d", len(txt.Txt))
	}
	if got := txt.Txt[0]; got != conf.SinkholeTXT {
		t.Fatalf("expected %s, got %s", conf.SinkholeTXT, got)
	}
}

func checkDomainTarget(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	var actual string
	switch rr := resp.Answer[0].(type) {
	case *dns.CNAME:
		actual = rr.Target
	case *dns.MX:
		actual = rr.Mx
	case *dns.NS:
		actual = rr.Ns
	case *dns.SRV:
		actual = rr.Target
	case *dns.PTR:
		actual = rr.Ptr
	default:
		t.Fatalf("unexpected type %T", resp.Answer[0])
	}
	if want := conf.SinkholeDomain + "."; actual != want {
		t.Fatalf("expected %s, got %s", want, actual)
	}
}

func checkSOA(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	soa, ok := resp.Answer[0].(*dns.SOA)
	if !ok {
		t.Fatalf("expected *dns.SOA, got %T", resp.Answer[0])
	}
	if got := soa.Ns; got != conf.SinkholeDomain+"." {
		t.Fatalf("expected localhost., got %s", got)
	}
	if got := soa.Mbox; got != fmt.Sprintf("hostmaster.%s.", conf.SinkholeDomain) {
		t.Fatalf("expected hostmaster.%s., got %s", conf.SinkholeDomain, got)
	}
}

func checkCAA(t *testing.T, resp *dns.Msg, conf Config) {
	t.Helper()
	caa, ok := resp.Answer[0].(*dns.CAA)
	if !ok {
		t.Fatalf("expected *dns.CAA, got %T", resp.Answer[0])
	}
	if got := caa.Value; got != conf.SinkholeDomain {
		t.Fatalf("expected %s, got %s", conf.SinkholeDomain, got)
	}
	if got := caa.Tag; got != "issue" {
		t.Fatalf("expected tag issue, got %s", got)
	}
}

func TestQueryOverUDPAndTCP(t *testing.T) {
	udpClient, tcpClient, addr, config, teardown := queryBothTransportsHelper(t)
	defer teardown()

	assertA := func(resp *dns.Msg) {
		t.Helper()
		if len(resp.Answer) != 1 {
			t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
		}
		a, ok := resp.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("expected *dns.A, got %T", resp.Answer[0])
		}
		if got := a.A.String(); got != config.SinkholeIPv4.String() {
			t.Fatalf("expected %s, got %s", config.SinkholeIPv4.String(), got)
		}
	}

	respUDP := exchange(t, udpClient, addr, "example.com.", dns.TypeA)
	respTCP := exchange(t, tcpClient, addr, "example.com.", dns.TypeA)

	assertA(respUDP)
	assertA(respTCP)
}

func exchange(t *testing.T, client *dns.Client, addr, name string, qtype uint16) *dns.Msg {
	t.Helper()

	m := new(dns.Msg)
	m.SetQuestion(name, qtype)

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, _, err := client.Exchange(m, addr)
		if err == nil && resp != nil {
			return resp
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no response")
	}
	t.Fatalf("dns exchange failed: %v", lastErr)
	return nil
}
