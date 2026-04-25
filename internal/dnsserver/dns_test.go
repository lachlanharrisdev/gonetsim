package dnsserver

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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

func TestAQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeA)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	a, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A, got %T", response.Answer[0])
	}
	if got := a.A.String(); got != config.SinkholeIPv4.String() {
		t.Fatalf("expected %s, got %s", config.SinkholeIPv4.String(), got)
	}
}

func TestAAAAQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeAAAA)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	aaaa, ok := response.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("expected *dns.AAAA, got %T", response.Answer[0])
	}
	if got := aaaa.AAAA.String(); got != config.SinkholeIPv6.String() {
		t.Fatalf("expected %s, got %s", config.SinkholeIPv6.String(), got)
	}
}

func TestTXTQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeTXT)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	txt, ok := response.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("expected *dns.TXT, got %T", response.Answer[0])
	}
	if len(txt.Txt) != 1 {
		t.Fatalf("expected 1 TXT record, got %d", len(txt.Txt))
	}
	if got := txt.Txt[0]; got != config.SinkholeTXT {
		t.Fatalf("expected %s, got %s", config.SinkholeTXT, got)
	}
}

func TestCNAMEQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeCNAME)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	cname, ok := response.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("expected *dns.CNAME, got %T", response.Answer[0])
	}
	if got := cname.Target; got != config.SinkholeDomain+"." {
		t.Fatalf("expected %s., got %s", config.SinkholeDomain, got)
	}
}

func TestMXQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeMX)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	mx, ok := response.Answer[0].(*dns.MX)
	if !ok {
		t.Fatalf("expected *dns.MX, got %T", response.Answer[0])
	}
	if got := mx.Mx; got != config.SinkholeDomain+"." {
		t.Fatalf("expected %s., got %s", config.SinkholeDomain, got)
	}
}

func TestNSQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeNS)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	ns, ok := response.Answer[0].(*dns.NS)
	if !ok {
		t.Fatalf("expected *dns.NS, got %T", response.Answer[0])
	}
	if got := ns.Ns; got != config.SinkholeDomain+"." {
		t.Fatalf("expected %s., got %s", config.SinkholeDomain, got)
	}
}

func TestSRVQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "_sip._tcp.example.com.", dns.TypeSRV)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	srv, ok := response.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("expected *dns.SRV, got %T", response.Answer[0])
	}
	if got := srv.Target; got != config.SinkholeDomain+"." {
		t.Fatalf("expected %s., got %s", config.SinkholeDomain, got)
	}
}

func TestPTRQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypePTR)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	ptr, ok := response.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("expected *dns.PTR, got %T", response.Answer[0])
	}
	if got := ptr.Ptr; got != config.SinkholeDomain+"." {
		t.Fatalf("expected %s., got %s", config.SinkholeDomain, got)
	}
}

func TestSOAQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeSOA)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(response.Answer))
	}
	soa, ok := response.Answer[0].(*dns.SOA)
	if !ok {
		t.Fatalf("expected *dns.SOA, got %T", response.Answer[0])
	}
	if got := soa.Ns; got != config.SinkholeDomain+"." {
		t.Fatalf("expected localhost., got %s", got)
	}
	if got := soa.Mbox; got != fmt.Sprintf("hostmaster.%s.", config.SinkholeDomain) {
		t.Fatalf("expected hostmaster.%s., got %s", config.SinkholeDomain, got)
	}
}

func TestCAAQuery(t *testing.T) {
	client, addr, config, teardown := queryTestsHelper(t)
	defer teardown()

	response := exchange(t, client, addr, "example.com.", dns.TypeCAA)
	if len(response.Answer) != 1 {
		t.Fatalf("expected 1 answer, god %d", len(response.Answer))
	}
	caa, ok := response.Answer[0].(*dns.CAA)
	if !ok {
		t.Fatalf("expected *dns.CAA, got %T", response.Answer[0])
	}
	if got := caa.Value; got != config.SinkholeDomain {
		t.Fatalf("expected %s, got %s", config.SinkholeDomain, got)
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
