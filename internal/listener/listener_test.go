////----------------------------------------------------------------------------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
//
// For more information please see `/.github/AI_USAGE.md`
//----------------------------------------------------------------------------//

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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func testLogger() *slog.Logger {
	return testutil.Logger()
}

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

func freePort(t *testing.T, network string) string {
	t.Helper()
	return testutil.FreePort(t, network)
}

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
	}
}

// waitTransportFrames polls the capture until the transport payload sequence
// matches want, tolerating the async flush that follows connection teardown.
func waitTransportFrames(t *testing.T, path, proto string, want []string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := transportPayloads(path, proto)
		if err == nil && strings.Join(got, "|") == strings.Join(want, "|") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := transportPayloads(path, proto)
	t.Fatalf("payload sequence never matched %q (proto %s), last saw %q", strings.Join(want, "|"), proto, strings.Join(got, "|"))
}

// waitSubstringFrames polls until the concatenated payload sequence of a
// capture contains want (used where multiple datagrams share one writer).
func waitSubstringFrames(t *testing.T, path, proto, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := transportPayloads(path, proto)
		if err == nil && strings.Contains(strings.Join(got, "|"), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := transportPayloads(path, proto)
	t.Fatalf("payload sequence never contained %q (proto %s), last saw %q", want, proto, strings.Join(got, "|"))
}

// transportPayloads extracts transport-layer payloads from a pcapng file,
// or an error if the file is empty or unreadable.
func transportPayloads(path, proto string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		return nil, err
	}
	var out []string
	for {
		data, _, err := r.ReadPacketData()
		if err != nil {
			break
		}
		pkt := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)
		var payload []byte
		if proto == "udp" {
			if u, ok := pkt.Layer(layers.LayerTypeUDP).(*layers.UDP); ok {
				payload = u.Payload
			}
		} else if t, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			payload = t.Payload
		}
		out = append(out, string(payload))
	}
	return out, nil
}

func TestTCPService(t *testing.T) {
	t.Run("echo over tcp with pcapng capture", func(t *testing.T) {
		conf := echoConfig(t)
		run, path := testRun(t)

		svc, err := NewService(conf, nil, testLogger(), run)
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

		// produce a pcapng file with the exchanged payload
		if _, err := transportPayloads(path, "tcp"); err != nil {
			t.Fatalf("capture: %v", err)
		}
	})

	t.Run("idle timeout closes connection", func(t *testing.T) {
		conf := echoConfig(t)
		conf.ReadTimeout = 200 * time.Millisecond

		svc, err := NewService(conf, nil, testLogger(), nil)
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
		conf := Config{
			Name:        "isotest",
			Network:     "tcp",
			Addr:        freePort(t, "tcp"),
			HandlerSpec: "lua:isolated.lua",
			BaseDir:     "../handler/testdata",
			ReadTimeout: 5 * time.Second,
		}
		svc, err := NewService(conf, nil, testLogger(), nil)
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

func TestTCPCapture(t *testing.T) {
	t.Run("tcp listener produces pcapng with frames", func(t *testing.T) {
		conf := echoConfig(t)
		run, path := testRun(t)

		svc, err := NewService(conf, nil, testLogger(), run)
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
		_ = conn.Close()

		waitTransportFrames(t, path, "tcp",
			[]string{"", "", "hello\n", "hello\n", "", ""})
	})
}

func TestTCPServiceTLS(t *testing.T) {
	t.Run("echo over TLS", func(t *testing.T) {
		conf := echoConfig(t)
		conf.TLS = &tlsprovider.Config{}

		svc, err := NewService(conf, nil, testLogger(), nil)
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
}

func TestUDPCapture(t *testing.T) {
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

	t.Run("udp echo produces pcapng", func(t *testing.T) {
		conf := Config{
			Name:        "udpecho",
			Network:     "udp",
			Addr:        freePort(t, "udp"),
			HandlerSpec: "builtin:echo",
			ReadTimeout: 150 * time.Millisecond,
			Capture:     true,
		}
		run, path := testRun(t)
		svc, err := NewService(conf, nil, testLogger(), run)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)
		exchange(t, conf.Addr, "ping", "ping")

		waitSubstringFrames(t, path, "udp", "ping|ping")
	})

	t.Run("udp lua packets", func(t *testing.T) {
		conf := Config{
			Name:        "udplua",
			Network:     "udp",
			Addr:        freePort(t, "udp"),
			HandlerSpec: "lua:packet.lua",
			BaseDir:     "../handler/testdata",
			ReadTimeout: 5 * time.Second,
		}
		svc, err := NewService(conf, nil, testLogger(), nil)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		startService(t, svc)
		exchange(t, conf.Addr, "ping", "pong")
	})
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
		svc, err := NewService(conf, nil, testLogger(), nil)
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
			_, err := NewService(conf, nil, testLogger(), nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}
