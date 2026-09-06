////----------------------------------------------------------------------------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
//
// For more information please see `/.github/AI_USAGE.md`
//----------------------------------------------------------------------------//

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

	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
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

	logger := testutil.Logger()
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

	logger := testutil.Logger()
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
	_, resp := testutil.RetryGet(t, client, url)
	return resp
}

func portFromAddr(t *testing.T, addr string) string {
	t.Helper()
	return testutil.MustPort(t, addr)
}

// tempDirWithFiles creates a temporary directory, writes the given files into it,
// and returns the directory path. Callers must defer os.RemoveAll on the result.
func tempDirWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		// support subdirectories in file names
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	return dir
}

// startRealServer spins up a real-mode HTTP server on a random port,
// returns the server and its base URL. Caller must defer shutdown.
func startRealServer(t *testing.T, rootDir string, statusCode int) (*http.Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	logger := testutil.Logger()
	srv, err := NewServer(Config{
		Addr:       "127.0.0.1:0",
		Mode:       "real",
		RootDir:    rootDir,
		StatusCode: statusCode,
	}, nil, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	go func() { _ = srv.Serve(ln) }()

	return srv, "http://" + ln.Addr().String()
}

// --- happy-path tests ---

func TestRealHandler_ServesHTMLFile(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"index.html": "<html><body>hello world</body></html>",
	})
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/index.html")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html Content-Type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hello world") {
		t.Fatalf("expected body to contain 'hello world', got %q", string(body))
	}
}

func TestRealHandler_StatusCodeOverride(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"page.html": "<html>ok</html>",
	})
	_, base := startRealServer(t, dir, http.StatusCreated)

	resp := mustGet(t, http.DefaultClient, base+"/page.html")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

// --- not-found / error tests ---

func TestRealHandler_MissingFileReturns404(t *testing.T) {
	dir := t.TempDir() // empty
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/nonexistent.html")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- security tests ---

func TestRealHandler_TraversalBlocked(t *testing.T) {
	// Single server; sentinel file lives outside the root.
	parent := t.TempDir()
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret contents"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	root := filepath.Join(parent, "www")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	_, base := startRealServer(t, root, 0)

	for _, path := range []string{"/../secret.txt", "/%2e%2e/secret.txt", "/a/../secret.txt", "/../../secret.txt"} {
		resp := mustGet(t, http.DefaultClient, base+path)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		// Mmst not serve the file, 404 or 400 are both accepted
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("traversal %q succeeded — got 200 with body: %q", path, string(body))
		}
	}
}

// --- config validation tests ---

func TestNewServer_RealMode_MissingRootDirReturnsError(t *testing.T) {
	logger := testutil.Logger()
	_, err := NewServer(Config{
		Addr: "127.0.0.1:0",
		Mode: "real",
		// RootDir intentionally omitted
	}, nil, logger)
	if err == nil {
		t.Fatal("expected error when RootDir is empty, got nil")
	}
}
