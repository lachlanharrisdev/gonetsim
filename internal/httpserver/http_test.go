package httpserver

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

// / <summary>
// / "smoke" test for http server. starts server, makes a request, inspects response & shuts down server
// / </summary>
func TestHTTPServer_Smoke(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// failed to listen on a local port with error
		t.Fatalf("listen: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(Config{Addr: "127.0.0.1:0", StatusCode: http.StatusCreated}, nil, logger)
	if err != nil {
		// failed to create server with error
		t.Fatalf("New: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	url := "http://" + ln.Addr().String() + "/hello"
	resp := mustGet(t, http.DefaultClient, url)

	defer func() {
		if err := resp.Body.Close(); err != nil {
			// failed to close response body with error
			t.Fatalf("close: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		// failed to get expected status code
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		// failed to get expected content type
		t.Fatalf("expected Content-Type text/html, got %q", ct)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if !strings.Contains(string(body), "GoNetSim HTTP Server") {
		// failed to get expected body content
		t.Fatalf("expected response body to contain HTML page content, got %q", string(body))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	select {
	case <-errCh:
	case <-time.After(1 * time.Second):
		// failed to shut down server cleanly
		t.Fatalf("server did not exit")
	}
}

// / <summary>
// / same smoke test but for the https server. starts server with self-signed cert, makes a request, inspects response & shuts down server
// / </summary>
func TestHTTPSServer_Smoke(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// failed to listen on a local port with error
		t.Fatalf("listen: %v", err)
	}

	cert, err := tlsprovider.GenerateSelfSigned(tlsprovider.SelfSignedOptions{DNSNames: []string{"localhost"}})
	if err != nil {
		// failed to generate self-signed certificate with error
		t.Fatalf("GenerateSelfSigned: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(Config{Addr: "127.0.0.1:0", StatusCode: http.StatusOK}, nil, logger)
	if err != nil {
		// failed to create https server with error
		t.Fatalf("New: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	errCh := make(chan error, 1)
	go func() {
		//  pass in-memory certs w/o temp files
		errCh <- srv.ServeTLS(ln, "", "")
	}()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	url := "https://localhost:" + portFromAddr(t, ln.Addr().String()) + "/secure"
	resp := mustGet(t, client, url)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// failed to close response body with error
			t.Fatalf("close: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// failed to get expected status code from https server
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected Content-Type text/html, got %q", ct)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if !strings.Contains(string(body), "fake mode") {
		t.Fatalf("expected response body to contain fake mode content, got %q", string(body))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	select {
	case <-errCh:
	case <-time.After(1 * time.Second):
		// failed to shut down https server cleanly
		t.Fatalf("https server did not exit")
	}
}

func mustGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			return resp
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("GET %s: %v", url, lastErr)
	return nil
}

func portFromAddr(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	return port
}
