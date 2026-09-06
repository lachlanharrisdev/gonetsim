////----------------------------------------------------------------------------
// NOTICE: to save development time, test files (including this) have been
// generated with LLMs. The author(s) do not claim credit for these tests
// and exist purely for maximising code quality and reliability
//
// For more information please see `/.github/AI_USAGE.md`
//----------------------------------------------------------------------------//

package handler

import (
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/lachlanharrisdev/gonetsim/internal/state"
	"github.com/lachlanharrisdev/gonetsim/internal/testutil"
)

func testLogger() *slog.Logger {
	return testutil.Logger()
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
		client, done := servePipe(t, EchoHandler{}, Env{Logger: testLogger()})
		roundtrip(t, client, "abc", "abc")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
	})

	t.Run("udp echo", func(t *testing.T) {
		addr, _ := net.ResolveUDPAddr("udp", "203.0.113.10:53")
		reply, err := EchoHandler{}.HandleUDP(t.Context(), []byte("query"), addr, Env{Logger: testLogger()})
		if err != nil || string(reply) != "query" {
			t.Fatalf("udp echo: %v %q", err, reply)
		}
	})
}

func TestNewSpecErrors(t *testing.T) {
	for _, spec := range []string{"", "noscheme", "builtin:nope", "python:foo.py", "lua:missing.lua", "lua:bad_syntax.lua", "lua:no_entry.lua", "lua:sandbox_escape.lua"} {
		if _, err := New(spec, "testdata", nil); err == nil {
			t.Errorf("New(%q): expected error", spec)
		}
	}
}

func TestLuaHandler(t *testing.T) {
	t.Run("tcp line roundtrip", func(t *testing.T) {
		h, err := NewLua("testdata/line_echo.lua", nil)
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		client, done := servePipe(t, h, Env{Logger: testLogger()})
		roundtrip(t, client, "hello\nworld\n", "echo: hello\necho: world\n")
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
	})

	t.Run("udp packets", func(t *testing.T) {
		remote, _ := net.ResolveUDPAddr("udp", "203.0.113.10:53531")
		h, err := NewLua("testdata/packet.lua", nil)
		if err != nil {
			t.Fatalf("NewLua: %v", err)
		}
		reply, err := h.HandleUDP(t.Context(), []byte("ping"), remote, Env{Logger: testLogger()})
		if err != nil || string(reply) != "pong" {
			t.Fatalf("ping: %v %q", err, reply)
		}
		reply, err = h.HandleUDP(t.Context(), []byte("other"), remote, Env{Logger: testLogger()})
		if err != nil || reply != nil {
			t.Fatalf("silent: %v %q", err, reply)
		}
	})
}

func TestLuaState(t *testing.T) {
	h, err := NewLua("testdata/state.lua", state.NewBudget(state.DefaultTotalLimit))
	if err != nil {
		t.Fatalf("NewLua: %v", err)
	}
	env := Env{Logger: testLogger(), Global: state.NewStore(state.NewBudget(state.DefaultTotalLimit))}
	for i, want := range []string{"1|conn|yes", "2|conn|yes"} {
		client, done := servePipe(t, h, env)
		buf := make([]byte, len(want))
		if _, err := io.ReadFull(client, buf); err != nil {
			t.Fatalf("ReadFull: %v", err)
		}
		if string(buf) != want {
			t.Fatalf("connection %d = %q, want %q", i, buf, want)
		}
		_ = client.Close()
		if err := <-done; err != nil {
			t.Fatalf("HandleTCP: %v", err)
		}
	}
}

func TestSandboxGlobals(t *testing.T) {
	h, err := NewLua("testdata/sandbox_report.lua", nil)
	if err != nil {
		t.Fatalf("NewLua: %v", err)
	}
	client, done := servePipe(t, h, Env{Logger: testLogger()})
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
}
