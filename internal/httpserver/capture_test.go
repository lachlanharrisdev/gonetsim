// //----------------------------------------------------------------------------
// // NOTICE: to save development time, test files (including this) have been
// // generated with LLMs. The author(s) do not claim credit for these tests
// // and exist purely for maximising code quality and reliability
// //
// // For more information please see `/.github/AI_USAGE.md`
// //----------------------------------------------------------------------------//

package httpserver

import (
	"context"
	"crypto/tls"
	"io"
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

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
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

func TestService_CapturesHTTP(t *testing.T) {
	conf := Config{
		Addr:       freeTCPAddr(t),
		StatusCode: http.StatusOK,
		Mode:       "fake",
		Capture:    true,
	}
	run, path := testRun(t)
	svc, errCh := startHTTPService(t, conf, run)

	get := func(url string) *http.Response {
		_, resp := retryingGet(t, http.DefaultClient, url)
		return resp
	}
	get("http://" + conf.Addr + "/warmup")
	resp := get("http://" + conf.Addr + "/hello")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	waitHTTPCapture(t, path, "GET /hello")
	waitTCPPayloadsContain(t, path, "HTTP/1.1 200")

	svc.Stop(context.Background()) //nolint:errcheck,gosec
	discardStartErr(t, errCh)
}

func TestService_CapturesHTTPS(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, _, err := tlsprovider.GenerateSelfSignedWithCA(tlsprovider.SelfSignedOptions{DNSNames: []string{"localhost"}})
	if err != nil {
		t.Fatalf("GenerateSelfSignedWithCA: %v", err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	conf := Config{
		Addr:       freeTCPAddr(t),
		StatusCode: http.StatusOK,
		Mode:       "fake",
		TLS:        &tlsprovider.Config{CertFile: certPath, KeyFile: keyPath},
		Capture:    true,
	}
	run, path := testRun(t)
	svc, errCh := startHTTPService(t, conf, run)

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	_, resp := retryingGet(t, client, "https://localhost:"+portOf(t, conf.Addr)+"/warmup")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	_, resp = retryingGet(t, client, "https://localhost:"+portOf(t, conf.Addr)+"/secure")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	// TLS capture is ciphertext, so assert the run capture holds TLS
	// records in both directions rather than plaintext content.
	waitHTTPCapture(t, path, "")
	waitTCPPayloads(t, path, func(joined string) bool {
		return strings.Contains(joined, "\x16") && strings.Contains(joined, "\x17")
	})

	svc.Stop(context.Background()) //nolint:errcheck,gosec
	discardStartErr(t, errCh)
}

// retryingGet issues a GET with Connection: close (forcing the server to
// close the connection so its capture is flushed), retrying while the service
// is still starting up.
func retryingGet(t *testing.T, client *http.Client, url string) (status int, resp *http.Response) {
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

func startHTTPService(t *testing.T, conf Config, run *capture.Run) (service.Service, <-chan error) {
	t.Helper()
	logger := testutil.Logger()
	svc := NewService(conf, logger, run)

	errCh := make(chan error, 1)
	go func() { errCh <- svc.Start(context.Background()) }()
	return svc, errCh
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

// waitHTTPCapture waits for the run capture's payloads to contain want
// (or for any packets when want is empty).
func waitHTTPCapture(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if payloads, err := tcpPayloads(path); err == nil && (want == "" || strings.Contains(payloads, want)) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run capture %s never contained %q", path, want)
}

func waitTCPPayloadsContain(t *testing.T, path, want string) {
	t.Helper()
	waitTCPPayloads(t, path, func(joined string) bool {
		return strings.Contains(joined, want)
	})
}

func waitTCPPayloads(t *testing.T, path string, cond func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		joined, err := tcpPayloads(path)
		if err == nil && cond(joined) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	joined, _ := tcpPayloads(path)
	t.Fatalf("capture %s never satisfied predicate (payloads %q)", path, joined)
}

func tcpPayloads(path string) (string, error) {
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
		if t, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			sb.Write(t.Payload)
		}
	}
	return sb.String(), nil
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func portOf(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	return port
}
