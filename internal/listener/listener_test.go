package listener

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// freePort reserves an ephemeral port, releases it, and returns its address.
func freePort(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket: %v", err)
		}
		addr := pc.LocalAddr().String()
		_ = pc.Close()
		return addr
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// startService runs svc in the background; cleanup cancels it and waits for
// Start to return.
func startService(t *testing.T, svc service.Service) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- svc.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("service start error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("service did not stop within 5s")
		}
	})
}

func dialTCP(t *testing.T, addr string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("dial %s failed: %v", addr, lastErr)
	return nil
}

func dialTLS(t *testing.T, addr, serverName string) net.Conn {
	t.Helper()
	tlsConf := &tls.Config{InsecureSkipVerify: true, ServerName: serverName}
	deadline := time.Now().Add(2 * time.Second)
	var conn net.Conn
	var lastErr error
	for time.Now().Before(deadline) {
		conn, lastErr = tls.Dial("tcp", addr, tlsConf)
		if lastErr == nil {
			return conn
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tls.Dial %s failed: %v", addr, lastErr)
	return nil
}

func echoConfig(t *testing.T) Config {
	return Config{
		Name:        "echotest",
		Network:     "tcp",
		Addr:        freePort(t, "tcp"),
		HandlerSpec: "builtin:echo",
		ReadTimeout: 5 * time.Second,
		Capture:     true,
		CaptureDir:  t.TempDir(),
	}
}

func captureFile(t *testing.T, dir, listener string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(filepath.Join(dir, listener))
		if err == nil && len(entries) > 0 {
			data, err := os.ReadFile(filepath.Join(dir, listener, entries[0].Name()))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			return string(data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no capture file appeared in %s", dir)
	return ""
}

func luaConfig(t *testing.T, name, script string) Config {
	return Config{
		Name:        name,
		Network:     "tcp",
		Addr:        freePort(t, "tcp"),
		HandlerSpec: "lua:" + script,
		BaseDir:     "../handler/testdata",
		ReadTimeout: 5 * time.Second,
	}
}

func TestTCPService(t *testing.T) {
	t.Run("echo and capture", func(t *testing.T) {
		dir := t.TempDir()
		conf := echoConfig(t)
		conf.CaptureDir = dir

		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)

		conn := dialTCP(t, conf.Addr)
		if _, err := conn.Write([]byte("hello\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		buf := make([]byte, 6)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "hello\n" {
			t.Fatalf("expected echo, got %q", buf)
		}
		_ = conn.Close()

		if got := captureFile(t, dir, conf.Name); got != "hello\n" {
			t.Fatalf("capture content %q", got)
		}
	})

	t.Run("idle timeout closes connection", func(t *testing.T) {
		conf := echoConfig(t)
		conf.ReadTimeout = 200 * time.Millisecond

		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)

		conn := dialTCP(t, conf.Addr)
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		buf := make([]byte, 1)
		if _, err := conn.Read(buf); err == nil {
			t.Fatalf("expected read to fail after idle timeout")
		}
	})

	t.Run("script errors don't kill the listener", func(t *testing.T) {
		conf := luaConfig(t, "isotest", "isolated.lua")
		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)

		conn := dialTCP(t, conf.Addr)
		_, _ = conn.Write([]byte("boom\n"))
		_ = conn.Close()

		conn = dialTCP(t, conf.Addr)
		defer func() { _ = conn.Close() }()
		if _, err := conn.Write([]byte("fine\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 3)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "ok\n" {
			t.Fatalf("expected ok, got %q", buf)
		}
	})
}

func TestTCPServiceTLS(t *testing.T) {
	t.Run("echo over TLS", func(t *testing.T) {
		conf := echoConfig(t)
		conf.TLS = &tlsprovider.Config{}

		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)

		conn := dialTLS(t, conf.Addr, "localhost")
		defer func() { _ = conn.Close() }()
		if _, err := conn.Write([]byte("secure")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		buf := make([]byte, 6)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "secure" {
			t.Fatalf("expected echo, got %q", buf)
		}
	})

	t.Run("SNI visible to script", func(t *testing.T) {
		conf := luaConfig(t, "snitest", "sni.lua")
		conf.TLS = &tlsprovider.Config{}

		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)

		conn := dialTLS(t, conf.Addr, "c2.evil.example")
		defer func() { _ = conn.Close() }()
		buf := make([]byte, len("sni:c2.evil.example"))
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != "sni:c2.evil.example" {
			t.Fatalf("expected SNI reply, got %q", buf)
		}
	})
}

func TestUDPService(t *testing.T) {
	exchange := func(t *testing.T, addr, payload, want string) {
		t.Helper()
		server, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			t.Fatalf("ResolveUDPAddr: %v", err)
		}
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket: %v", err)
		}
		defer func() { _ = pc.Close() }()

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := pc.WriteTo([]byte(payload), server); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}
			_ = pc.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			buf := make([]byte, 64)
			n, _, err := pc.ReadFrom(buf)
			if err == nil {
				if string(buf[:n]) != want {
					t.Fatalf("expected %q, got %q", want, buf[:n])
				}
				return
			}
		}
		t.Fatalf("no reply for %q", payload)
	}

	t.Run("echo", func(t *testing.T) {
		conf := Config{
			Name:        "udpecho",
			Network:     "udp",
			Addr:        freePort(t, "udp"),
			HandlerSpec: "builtin:echo",
			ReadTimeout: 5 * time.Second,
		}
		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)
		exchange(t, conf.Addr, "query", "query")
	})

	t.Run("lua packets", func(t *testing.T) {
		conf := Config{
			Name:        "udplua",
			Network:     "udp",
			Addr:        freePort(t, "udp"),
			HandlerSpec: "lua:packet.lua",
			BaseDir:     "../handler/testdata",
			ReadTimeout: 5 * time.Second,
		}
		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)
		exchange(t, conf.Addr, "ping", "pong")
	})
}

// TestCaptureStoreEviction verifies idle UDP writers are swept so capture
// files don't accumulate open handles for the life of the listener
func TestCaptureStoreEviction(t *testing.T) {
	cs, err := capture.NewStore(t.TempDir(), "evict")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store := &captureStore{store: cs, idle: 30 * time.Millisecond}
	t.Cleanup(func() { store.closeAll() })

	_, err = store.writer("10.0.0.1:1")
	if err != nil {
		t.Fatalf("writer a: %v", err)
	}
	time.Sleep(60 * time.Millisecond)

	if _, err := store.writer("10.0.0.2:2"); err != nil { // sweeps the idle writer
		t.Fatalf("writer b: %v", err)
	}
	if len(store.entries) != 1 {
		t.Fatalf("expected idle writer to be evicted, %d entries remain", len(store.entries))
	}

	if _, err := store.writer("10.0.0.1:1"); err != nil {
		t.Fatalf("writer a again: %v", err)
	}
	if len(store.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(store.entries))
	}
}

func TestStartWithCancelledContext(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		conf := Config{
			Name:        "canceled-" + network,
			Network:     network,
			Addr:        freePort(t, network),
			HandlerSpec: "builtin:sink",
			ReadTimeout: 5 * time.Second,
		}
		svc, err := NewService(conf, testLogger())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() {
			done <- svc.Start(ctx)
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Start (%s): %v", network, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Start (%s) did not return after cancellation", network)
		}
	}
}

func TestNewServiceValidation(t *testing.T) {
	base := func() Config {
		return Config{
			Name:        "bad",
			Network:     "tcp",
			Addr:        "127.0.0.1:0",
			HandlerSpec: "builtin:echo",
			ReadTimeout: 5 * time.Second,
		}
	}

	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"unknown network", func(c *Config) { c.Network = "sctp" }, "network must be"},
		{"bad addr", func(c *Config) { c.Addr = "nope" }, "invalid listen addr"},
		{"missing handler", func(c *Config) { c.HandlerSpec = "" }, "handler is required"},
		{"bad timeout", func(c *Config) { c.ReadTimeout = -1 }, "read_timeout must be"},
		{"tls on udp", func(c *Config) {
			c.Network = "udp"
			c.TLS = &tlsprovider.Config{}
		}, "tls is only supported on tcp"},
		{"missing script", func(c *Config) { c.HandlerSpec = "lua:missing.lua" }, "read lua script"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := base()
			tc.mutate(&conf)
			_, err := NewService(conf, testLogger())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}
