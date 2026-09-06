package testutil

import (
	"io"
	"log/slog"
	"net"
	"net/http"
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
)

func Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func FreeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().String()
}

func FreePort(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket: %v", err)
		}
		defer func() { _ = pc.Close() }()
		return pc.LocalAddr().String()
	}
	return FreeTCPAddr(t)
}

func MustPort(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	return port
}

func NewPcapRun(t *testing.T) (*capture.Run, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.pcapng")
	run, err := capture.NewRun(path)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	t.Cleanup(func() { _ = run.Close() })
	return run, path
}

func WaitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting: %s", msg)
}

func TransportPayloads(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
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
		} else if tc, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			sb.Write(tc.Payload)
		}
	}
	return sb.String(), nil
}

func WaitForPayload(t *testing.T, path string, timeout time.Duration, cond func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if joined, err := TransportPayloads(path); err == nil && cond(joined) {
			return joined
		}
		time.Sleep(20 * time.Millisecond)
	}
	joined, _ := TransportPayloads(path)
	t.Fatalf("capture %s never satisfied predicate (payloads %q)", path, joined)
	return ""
}

func WaitForPayloadContains(t *testing.T, path, want string, timeout time.Duration) {
	t.Helper()
	WaitForPayload(t, path, timeout, func(s string) bool {
		return want == "" || strings.Contains(s, want)
	})
}

func DiscardServiceStartErr(t *testing.T, errCh <-chan error) {
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

func RetryGet(t *testing.T, client *http.Client, url string) (int, *http.Response) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		req.Header.Set("Connection", "close")
		r, err := client.Do(req)
		if err == nil {
			return r.StatusCode, r
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("GET %s: %v", url, lastErr)
	return 0, nil
}

func RetryDNSExchange(t *testing.T, client *dns.Client, addr string, m *dns.Msg) (*dns.Msg, time.Duration, error) {
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
