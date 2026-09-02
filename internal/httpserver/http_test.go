// --------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
// --------

package httpserver

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
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
	defer resp.Body.Close()

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

func TestRealHandler_ServesTextFile(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"readme.txt": "this is a plain text file",
	})
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/readme.txt")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain Content-Type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "plain text file") {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestRealHandler_ServesBinaryFile(t *testing.T) {
	// A minimal valid PNG (1x1 pixel, transparent)
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), pngBytes, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/pixel.png")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("expected image/png Content-Type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != len(pngBytes) {
		t.Fatalf("expected %d bytes, got %d", len(pngBytes), len(body))
	}
}

func TestRealHandler_ServesFileFromSubdirectory(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"assets/style.css": "body { color: red; }",
	})
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/assets/style.css")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "color: red") {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestRealHandler_RootPathServesIndexHTML(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"index.html": "<html><body>root index</body></html>",
	})
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/")
	defer resp.Body.Close()

	// / should fall through to index.html
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "root index") {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestRealHandler_StatusCodeOverride(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"page.html": "<html>ok</html>",
	})
	_, base := startRealServer(t, dir, http.StatusCreated)

	resp := mustGet(t, http.DefaultClient, base+"/page.html")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

// --- not-found / error tests ---

func TestRealHandler_MissingFileReturns404(t *testing.T) {
	dir := t.TempDir() // empty
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/nonexistent.html")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRealHandler_DirectoryRequestWithoutIndexReturns404(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"sub/file.txt": "content",
	})
	_, base := startRealServer(t, dir, 0)

	// Request the subdirectory itself — without an index.html it should 404,
	// not serve a listing.
	resp := mustGet(t, http.DefaultClient, base+"/sub/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for directory request, got %d", resp.StatusCode)
	}
}

func TestRealHandler_DirectoryServesIndexHTML(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"sub/index.html": "<html><body>sub index</body></html>",
	})
	_, base := startRealServer(t, dir, 0)

	resp := mustGet(t, http.DefaultClient, base+"/sub/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "sub index") {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestRealHandler_ConditionalRequestNotOverridden(t *testing.T) {
	dir := tempDirWithFiles(t, map[string]string{
		"page.txt": "hello",
	})
	_, base := startRealServer(t, dir, http.StatusAccepted) // non-200 override

	// Request once to learn the Last-Modified, then re-request with
	// If-Modified-Since to trigger a 304. The configured status override must
	// not clobber the 304 Not Modified response.
	first := mustGet(t, http.DefaultClient, base+"/page.txt")
	lm := first.Header.Get("Last-Modified")
	_ = first.Body.Close()
	if lm == "" {
		t.Fatal("expected a Last-Modified header on the first response")
	}

	req, err := http.NewRequest(http.MethodGet, base+"/page.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-Modified-Since", lm)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304 for conditional request, got %d", resp.StatusCode)
	}
}

// --- security tests ---

func TestRealHandler_DirectoryTraversalBlocked(t *testing.T) {
	// Write a sentinel file one level above the root dir.
	parent := t.TempDir()
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret contents"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// The server root is a subdirectory; secret.txt is outside it.
	root := filepath.Join(parent, "www")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, base := startRealServer(t, root, 0)

	// Classic traversal attempt
	resp := mustGet(t, http.DefaultClient, base+"/../secret.txt")
	defer resp.Body.Close()

	// Must not serve the file — 404 or 400 are both acceptable
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("traversal succeeded — got 200 with body: %q", string(body))
	}
}

func TestRealHandler_EncodedTraversalBlocked(t *testing.T) {
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

	// URL-encoded traversal: %2e%2e = ".."
	// http.DefaultClient will usually normalise this, but worth having
	resp := mustGet(t, http.DefaultClient, base+"/%2e%2e/secret.txt")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("encoded traversal succeeded — got 200 with body: %q", string(body))
	}
}

func TestRealHandler_MiddlePathTraversalBlocked(t *testing.T) {
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

	// .. in the middle of the path must still be contained within root.
	resp := mustGet(t, http.DefaultClient, base+"/a/../secret.txt")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("middle traversal succeeded — got 200 with body: %q", string(body))
	}
}

func TestRealHandler_PlainTraversalPath(t *testing.T) {
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

	// Raw .. components that survive URL parsing.
	resp := mustGet(t, http.DefaultClient, base+"/../../secret.txt")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("raw traversal succeeded — got 200 with body: %q", string(body))
	}
}

// --- config validation tests ---

func TestNewServer_RealMode_MissingRootDirReturnsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewServer(Config{
		Addr: "127.0.0.1:0",
		Mode: "real",
		// RootDir intentionally omitted
	}, nil, logger)
	if err == nil {
		t.Fatal("expected error when RootDir is empty, got nil")
	}
}

func TestNewServer_RealMode_NonexistentRootDirReturnsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewServer(Config{
		Addr:    "127.0.0.1:0",
		Mode:    "real",
		RootDir: "/this/path/does/not/exist",
	}, nil, logger)
	if err == nil {
		t.Fatal("expected error for nonexistent RootDir, got nil")
	}
}
