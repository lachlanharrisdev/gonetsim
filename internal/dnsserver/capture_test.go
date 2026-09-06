// //----------------------------------------------------------------------------
// // NOTICE: to save development time, test files (including this) have been
// // generated with LLMs. The author(s) do not claim credit for these tests
// // and exist purely for maximising code quality and reliability
// //
// // For more information please see `/.github/AI_USAGE.md`
// //----------------------------------------------------------------------------//

package dnsserver

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/miekg/dns"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
)

func testRun(t *testing.T) (*capture.Run, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.pcapng")
	run, err := capture.NewRun(path)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	t.Cleanup(func() { _ = run.Close() })
	return run, path
}

func TestService_CapturesUDP(t *testing.T) {
	conf := baseCaptureConfig(t, "udp")
	conf.Capture = true
	run, path := testRun(t)

	svc, errCh := startDNSService(t, conf, run)

	query := newAQuery()
	client := &dns.Client{Net: "udp", Timeout: 1 * time.Second}
	_, _, err := retryExchange(t, client, conf.Addr, query)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	waitDNSCapture(t, path, "example")
	waitTransportPayloads(t, path, func(s string) bool {
		return strings.Count(s, "example") >= 2 // query + response
	})

	svc.Stop(context.Background()) //nolint:errcheck,gosec
	discardStartErr(t, errCh)
}

func TestService_CapturesTCP(t *testing.T) {
	conf := baseCaptureConfig(t, "tcp")
	conf.Capture = true
	run, path := testRun(t)

	svc, errCh := startDNSService(t, conf, run)

	query := newAQuery()
	client := &dns.Client{Net: "tcp", Timeout: 1 * time.Second}
	_, _, err := retryExchange(t, client, conf.Addr, query)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	waitDNSCapture(t, path, "example")
	waitTransportPayloads(t, path, func(s string) bool {
		return strings.Count(s, "example") >= 2 // query + response
	})

	svc.Stop(context.Background()) //nolint:errcheck,gosec
	discardStartErr(t, errCh)
}

func baseCaptureConfig(t *testing.T, network string) Config {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	_ = pc.Close()

	return Config{
		Addr:           fmt.Sprintf("127.0.0.1:%d", port),
		Net:            network,
		SinkholeIPv4:   netip.MustParseAddr("203.0.113.10"),
		SinkholeIPv6:   netip.MustParseAddr("2001:db8::10"),
		SinkholeDomain: "localhost",
		SinkholeTXT:    "test",
		TTL:            60,
		Compress:       false,
	}
}

func newAQuery() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	return m
}

func startDNSService(t *testing.T, conf Config, run *capture.Run) (service.Service, <-chan error) {
	t.Helper()
	logger := testutil.Logger()
	svc := NewService(conf, logger, run)

	errCh := make(chan error, 1)
	go func() { errCh <- svc.Start(context.Background()) }()
	return svc, errCh
}

func retryExchange(t *testing.T, client *dns.Client, addr string, m *dns.Msg) (*dns.Msg, time.Duration, error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	var lastRTT time.Duration
	for time.Now().Before(deadline) {
		resp, rtt, err := client.Exchange(m, addr)
		if err == nil && resp != nil {
			return resp, rtt, nil
		}
		lastErr, lastRTT = err, rtt
		time.Sleep(20 * time.Millisecond)
	}
	return nil, lastRTT, lastErr
}

func discardStartErr(t *testing.T, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("service.Start returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("service.Start never returned")
	}
}

// waitDNSCapture waits for the run capture's transport payloads to contain
// want.
func waitDNSCapture(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if payloads, err := transportPayloads(path); err == nil && strings.Contains(payloads, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run capture %s never contained %q", path, want)
}

// waitTransportPayloads polls until the transport payloads of the capture
// satisfy cond (tolerating the async flush on connection teardown).
func waitTransportPayloads(t *testing.T, path string, cond func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		joined, err := transportPayloads(path)
		if err == nil && cond(joined) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	joined, _ := transportPayloads(path)
	t.Fatalf("capture %s never satisfied predicate (payloads %q)", path, joined)
}

// transportPayloads concatenates UDP/TCP payloads from a pcapng file.
func transportPayloads(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for {
		data, _, err := r.ReadPacketData()
		if err != nil {
			break
		}
		pkt := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)
		if u, ok := pkt.Layer(layers.LayerTypeUDP).(*layers.UDP); ok {
			sb.Write(u.Payload)
		} else if tcp, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			sb.Write(tcp.Payload)
		}
	}
	return sb.String(), nil
}
