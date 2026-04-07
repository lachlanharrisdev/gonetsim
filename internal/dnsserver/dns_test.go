package dnsserver

import (
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// / <summary>
// / for now, tests that A/AAAA queries return the correct sinkhole IP, and that other types return NOERROR
// / </summary>
func TestDNSServer_SinkholeResponses(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		// failed to listen on a local udp port with error
		t.Fatalf("ListenPacket: %v", err)
	}
	addr := pc.LocalAddr().String()

	conf := Config{
		Addr:         addr,
		Net:          "udp",
		SinkholeIPv4: netip.MustParseAddr("203.0.113.10"),
		SinkholeIPv6: netip.MustParseAddr("2001:db8::10"),
	}
	srv, err := NewServer(conf)
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
	defer func() {
		err = srv.Shutdown()
		if err != nil {
			// failed to shutdown server with error
			t.Fatalf("Shutdown: %v", err)
		}
		select {
		case <-errCh:
		case <-time.After(500 * time.Millisecond):
		}
	}()

	client := &dns.Client{Net: "udp", Timeout: 1 * time.Second}

	respA := exchange(t, client, addr, "example.com.", dns.TypeA)
	if len(respA.Answer) != 1 {
		// failed to get expected number of answers for A query
		t.Fatalf("A: expected 1 answer, got %d", len(respA.Answer))
	}
	a, ok := respA.Answer[0].(*dns.A)
	if !ok {
		// failed to get expected record type for A query
		t.Fatalf("A: expected *dns.A, got %T", respA.Answer[0])
	}
	if got := a.A.String(); got != conf.SinkholeIPv4.String() {
		// failed to get expected sinkhole ip for A query
		t.Fatalf("A: expected %s, got %s", conf.SinkholeIPv4.String(), got)
	}

	respAAAA := exchange(t, client, addr, "example.com.", dns.TypeAAAA)
	if len(respAAAA.Answer) != 1 {
		// failed to get expected number of answers for AAAA query
		t.Fatalf("AAAA: expected 1 answer, got %d", len(respAAAA.Answer))
	}
	aaaa, ok := respAAAA.Answer[0].(*dns.AAAA)
	if !ok {
		// failed to get expected record type for AAAA query
		t.Fatalf("AAAA: expected *dns.AAAA, got %T", respAAAA.Answer[0])
	}
	if got := aaaa.AAAA.String(); got != conf.SinkholeIPv6.String() {
		// failed to get expected sinkhole ip for AAAA query
		t.Fatalf("AAAA: expected %s, got %s", conf.SinkholeIPv6.String(), got)
	}

	respTXT := exchange(t, client, addr, "example.com.", dns.TypeTXT)
	if len(respTXT.Answer) != 0 {
		// failed to get expected number of answers for TXT query
		t.Fatalf("TXT: expected 0 answers, got %d", len(respTXT.Answer))
	}
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
