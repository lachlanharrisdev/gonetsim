package handler

import (
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testCapture opens a capture writer in a temp dir; the returned func reads
// the capture file back.
func testCapture(t *testing.T) (*capture.Writer, func() string) {
	t.Helper()
	base := t.TempDir()
	store, err := capture.NewStore(base, "test")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	w, err := store.Conn("203.0.113.10:1", time.Now())
	if err != nil {
		t.Fatalf("Conn: %v", err)
	}
	return w, func() string {
		entries, err := os.ReadDir(filepath.Join(base, "test"))
		if err != nil || len(entries) != 1 {
			return ""
		}
		data, _ := os.ReadFile(filepath.Join(base, "test", entries[0].Name()))
		return string(data)
	}
}

// pipe returns a connected pair, closed on test cleanup.
func pipe(t *testing.T) (client, server net.Conn) {
	t.Helper()
	client, server = net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	return client, server
}

// servePipe runs h against one end of a pipe; the other end is returned for
// the test to act as the client.
func servePipe(t *testing.T, h Handler, env Env) (net.Conn, <-chan error) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	done := make(chan error, 1)
	go func() {
		done <- h.HandleTCP(t.Context(), server, env)
	}()
	return client, done
}

// roundtrip writes payload and expects reply back.
func roundtrip(t *testing.T, client net.Conn, payload, reply string) {
	t.Helper()
	if _, err := client.Write([]byte(payload)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, len(reply))
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(buf) != reply {
		t.Fatalf("unexpected reply %q, want %q", buf, reply)
	}
}

func TestBuiltins(t *testing.T) {
	t.Run("tcp echo", func(t *testing.T) {
		w, read := testCapture(t)
		client, done := servePipe(t, EchoHandler{}, Env{Logger: discardLogger(), Capture: w})
		roundtrip(t, client, "abc", "abc")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
		if got := read(); got != "abc" {
			t.Fatalf("capture content %q", got)
		}
	})

	t.Run("tcp sink", func(t *testing.T) {
		w, read := testCapture(t)
		client, done := servePipe(t, SinkHandler{}, Env{Logger: discardLogger(), Capture: w})
		roundtrip(t, client, "secret exfil", "")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
		if got := read(); got != "secret exfil" {
			t.Fatalf("capture content %q", got)
		}
	})

	addr, _ := net.ResolveUDPAddr("udp", "203.0.113.10:53")
	t.Run("udp echo", func(t *testing.T) {
		reply, err := EchoHandler{}.HandleUDP(t.Context(), []byte("query"), addr, Env{Logger: discardLogger()})
		if err != nil || string(reply) != "query" {
			t.Fatalf("udp echo: %v %q", err, reply)
		}
	})

	t.Run("udp sink", func(t *testing.T) {
		reply, err := SinkHandler{}.HandleUDP(t.Context(), []byte("x"), nil, Env{Logger: discardLogger()})
		if err != nil || reply != nil {
			t.Fatalf("udp sink: %v %q", err, reply)
		}
	})
}

func TestNewSpecErrors(t *testing.T) {
	cases := []struct {
		spec    string
		baseDir string
	}{
		{"", "testdata"},
		{"noscheme", "testdata"},
		{"builtin:nope", "testdata"},
		{"python:foo.py", "testdata"},
		{"lua:missing.lua", "testdata"},
		{"lua:bad_syntax.lua", "testdata"},
		{"lua:no_entry.lua", "testdata"},
		{"lua:sandbox_escape.lua", "testdata"},
	}
	for _, tc := range cases {
		if _, err := New(tc.spec, tc.baseDir); err == nil {
			t.Errorf("New(%q): expected error", tc.spec)
		}
	}
}

func TestLuaHandler(t *testing.T) {
	t.Run("tcp line roundtrip", func(t *testing.T) {
		h, err := NewLua("testdata/line_echo.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		roundtrip(t, client, "hello\nworld\n", "echo: hello\necho: world\n")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
	})

	t.Run("tcp read(n)", func(t *testing.T) {
		h, err := NewLua("testdata/read_n.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		roundtrip(t, client, "ABCDrest", "got:ABCD")
		_ = client.Close()
		<-done
	})

	t.Run("tcp read_until headers", func(t *testing.T) {
		h, err := NewLua("testdata/read_until.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		roundtrip(t, client, "GET / HTTP/1.1\r\nHost: x\r\n\r\nrest", "len:27")
		_ = client.Close()
		<-done
	})

	t.Run("udp packets", func(t *testing.T) {
		h, err := NewLua("testdata/packet.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		reply, err := h.HandleUDP(t.Context(), []byte("ping"), nil, Env{Logger: discardLogger()})
		if err != nil || string(reply) != "pong" {
			t.Fatalf("ping: %v %q", err, reply)
		}
		reply, err = h.HandleUDP(t.Context(), []byte("other"), nil, Env{Logger: discardLogger()})
		if err != nil || reply != nil {
			t.Fatalf("silent: %v %q", err, reply)
		}
	})

	t.Run("capture and log", func(t *testing.T) {
		h, err := NewLua("testdata/capture.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		w, read := testCapture(t)
		client, done := servePipe(t, h, Env{Logger: discardLogger(), Capture: w})
		roundtrip(t, client, "payload", "")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
		if got := read(); got != "=== section ===\npayload\n" {
			t.Fatalf("capture content %q", got)
		}
	})

	t.Run("sandbox globals", func(t *testing.T) {
		h, err := NewLua("testdata/sandbox_report.lua")
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: discardLogger()})
		buf := make([]byte, 1024)
		n, err := client.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		reply := string(buf[:n])
		if !strings.Contains(reply, "io=nil") || !strings.Contains(reply, "os=nil") || !strings.Contains(reply, "require=nil") {
			t.Fatalf("sandbox globals leaked: %q", reply)
		}
		_ = client.Close()
		<-done
	})
}

func TestLuaConnLimits(t *testing.T) {
	flood := func(client net.Conn) {
		go func() {
			buf := make([]byte, 4096)
			for {
				if _, err := client.Write(buf); err != nil {
					return
				}
			}
		}()
	}

	t.Run("read_line cap", func(t *testing.T) {
		client, server := pipe(t)
		flood(client)

		lc := newLuaConn(server)
		_, err := lc.readLine()
		if err == nil || !strings.Contains(err.Error(), "line exceeds") {
			t.Fatalf("expected line cap error, got: %v", err)
		}
	})

	t.Run("read_until cap", func(t *testing.T) {
		client, server := pipe(t)
		flood(client)

		lc := newLuaConn(server)
		_, err := lc.readUntil([]byte("\r\n"))
		if err == nil || !strings.Contains(err.Error(), "read exceeds") {
			t.Fatalf("expected read cap error, got: %v", err)
		}
	})

	t.Run("read_until across chunks", func(t *testing.T) {
		client, server := pipe(t)
		go func() {
			_, _ = client.Write([]byte("HEAD"))
			_, _ = client.Write([]byte("ER:X"))
			_, _ = client.Write([]byte("\r\n\r\n"))
		}()

		lc := newLuaConn(server)
		v, err := lc.readUntil([]byte("\r\n\r\n"))
		if err != nil || v != lua.LString("HEADER:X\r\n\r\n") {
			t.Fatalf("readUntil: %v %q", err, v)
		}
	})
}
