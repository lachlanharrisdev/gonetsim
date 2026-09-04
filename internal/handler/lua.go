package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

const (
	maxReadLen = 1 << 20 // caps read_line/read_until, 1 MiB
	maxSleep   = time.Hour

	tcpEntry = "handle"        // handle(conn)
	udpEntry = "handle_packet" // handle_packet(data) -> string | nil
)

// LuaHandler serves connections with a Lua script, compiled once and run
// fresh per connection so scripts share no state. Scripts are sandboxed;
// they reach the network only through conn, log and capture.
type LuaHandler struct {
	path  string
	proto *lua.FunctionProto
}

func NewLua(path string) (*LuaHandler, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lua script: %w", err)
	}
	proto, err := compile(src, filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("compile lua script %q: %w", path, err)
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	openLibs(L)
	if err := runChunk(L, proto); err != nil {
		return nil, fmt.Errorf("load lua script %q: %w", path, err)
	}
	if L.GetGlobal(tcpEntry) == lua.LNil && L.GetGlobal(udpEntry) == lua.LNil {
		return nil, fmt.Errorf("lua script %q defines neither %s(conn) nor %s(data)", path, tcpEntry, udpEntry)
	}

	return &LuaHandler{path: path, proto: proto}, nil
}

func (h *LuaHandler) HandleTCP(ctx context.Context, conn net.Conn, env Env) error {
	L := h.newState(env)
	defer L.Close()

	lc := newLuaConn(conn)
	if _, err := h.run(L, tcpEntry, 0, registerConn(L, lc, ctx, env)); err != nil {
		return fmt.Errorf("lua %s: %w", h.path, err)
	}
	return nil
}

func (h *LuaHandler) HandleUDP(_ context.Context, data []byte, _ net.Addr, env Env) ([]byte, error) {
	L := h.newState(env)
	defer L.Close()

	ret, err := h.run(L, udpEntry, 1, lua.LString(data))
	if err != nil {
		return nil, fmt.Errorf("lua %s: %w", h.path, err)
	}
	if ret[0] == lua.LNil {
		return nil, nil
	}
	s, ok := ret[0].(lua.LString)
	if !ok {
		return nil, fmt.Errorf("%s(data) must return a string or nil, got %s", udpEntry, ret[0].Type().String())
	}
	return []byte(s), nil
}

// run executes the script chunk, then calls the entrypoint with args.
func (h *LuaHandler) run(L *lua.LState, entry string, nret int, args ...lua.LValue) ([]lua.LValue, error) {
	if err := runChunk(L, h.proto); err != nil {
		return nil, err
	}
	fn := L.GetGlobal(entry)
	if fn == lua.LNil {
		return nil, fmt.Errorf("script does not define %s", entry)
	}
	if err := L.CallByParam(lua.P{Fn: fn, NRet: nret, Protect: true}, args...); err != nil {
		return nil, err
	}
	vals := make([]lua.LValue, nret)
	for i := range vals {
		vals[i] = L.Get(-nret + i)
	}
	L.Pop(nret)
	return vals, nil
}

func runChunk(L *lua.LState, proto *lua.FunctionProto) error {
	return L.CallByParam(lua.P{Fn: L.NewFunctionFromProto(proto), NRet: 0, Protect: true})
}

func compile(src []byte, name string) (*lua.FunctionProto, error) {
	chunk, err := parse.Parse(bytes.NewReader(src), name)
	if err != nil {
		return nil, err
	}
	return lua.Compile(chunk, name)
}

func (h *LuaHandler) newState(env Env) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	openLibs(L)
	registerLog(L, env.Logger)
	registerCapture(L, env.Capture)
	return L
}

// openLibs loads the sandbox-safe stdlib subset plus string.pack/unpack
// (a Lua 5.3 feature gopher-lua lacks).
func openLibs(L *lua.LState) {
	lua.OpenBase(L)
	lua.OpenString(L)
	lua.OpenTable(L)
	lua.OpenMath(L)

	str := L.GetGlobal("string").(*lua.LTable)
	L.SetField(str, "pack", L.NewFunction(luaPack))
	L.SetField(str, "unpack", L.NewFunction(luaUnpack))

	// base exposes filesystem helpers; drop them
	for _, name := range []string{"dofile", "loadfile", "require"} {
		L.SetGlobal(name, lua.LNil)
	}
}

func registerLog(L *lua.LState, logger *slog.Logger) {
	log := L.NewTable()
	for _, e := range []struct {
		name  string
		level slog.Level
	}{
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	} {
		fn := L.NewFunction(func(L *lua.LState) int {
			logger.Log(context.Background(), e.level, luaStrings(L))
			return 0
		})
		L.SetField(log, e.name, fn)
	}
	L.SetGlobal("log", log)

	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		logger.Info(luaStrings(L))
		return 0
	}))
}

func registerCapture(L *lua.LState, w *capture.Writer) {
	capture := L.NewTable()
	L.SetField(capture, "write", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		data := L.CheckString(3)
		w.Write(name, []byte(data))
		return 0
	}))
	L.SetGlobal("capture", capture)
}

func luaStrings(L *lua.LState) string {
	parts := make([]string, L.GetTop())
	for i := range parts {
		parts[i] = L.ToString(i + 1)
	}
	return strings.Join(parts, " ")
}

type tlsState interface {
	ConnectionState() tls.ConnectionState
	HandshakeContext(ctx context.Context) error
}

// luaConn wraps a net.Conn with a shared buffered reader so read and
// read_line never lose buffered data.
type luaConn struct {
	net.Conn
	br *bufio.Reader
}

func newLuaConn(conn net.Conn) *luaConn {
	return &luaConn{Conn: conn, br: bufio.NewReader(conn)}
}

func (lc *luaConn) tls() tlsState {
	tc, _ := lc.Conn.(tlsState)
	return tc
}

func (lc *luaConn) ConnectionState() tls.ConnectionState {
	if tc := lc.tls(); tc != nil {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}

func (lc *luaConn) HandshakeContext(ctx context.Context) error {
	if tc := lc.tls(); tc != nil {
		return tc.HandshakeContext(ctx)
	}
	return nil
}

func (lc *luaConn) read(n int) (lua.LValue, error) {
	buf := make([]byte, n)
	nr, err := lc.br.Read(buf)
	if nr > 0 {
		return lua.LString(buf[:nr]), nil
	}
	if errors.Is(err, io.EOF) {
		return lua.LNil, nil
	}
	return nil, err
}

func (lc *luaConn) readLine() (lua.LValue, error) {
	var sb strings.Builder
	for {
		chunk, err := lc.br.ReadSlice('\n')
		sb.Write(chunk)
		if err == nil {
			return lua.LString(sb.String()), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			if sb.Len() > maxReadLen {
				return nil, fmt.Errorf("line exceeds %d bytes", maxReadLen)
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			if sb.Len() > 0 {
				return lua.LString(sb.String()), nil
			}
			return lua.LNil, nil
		}
		return nil, err
	}
}

func (lc *luaConn) readUntil(delim []byte) (lua.LValue, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		if i := bytes.Index(buf, delim); i >= 0 {
			return lua.LString(buf[:i+len(delim)]), nil
		}
		if len(buf) > maxReadLen {
			return nil, fmt.Errorf("read exceeds %d bytes", maxReadLen)
		}
		n, err := lc.br.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(buf) > 0 {
					return lua.LString(buf), nil
				}
				return lua.LNil, nil
			}
			return nil, err
		}
	}
}

func registerConn(L *lua.LState, lc *luaConn, ctx context.Context, env Env) *lua.LTable {
	conn := L.NewTable()

	L.SetField(conn, "read", L.NewFunction(func(L *lua.LState) int {
		n := L.CheckInt(2)
		if n <= 0 {
			L.ArgError(2, "read size must be > 0")
			return 0
		}
		v, err := lc.read(n)
		return pushResult(L, v, err)
	}))

	L.SetField(conn, "read_line", L.NewFunction(func(L *lua.LState) int {
		v, err := lc.readLine()
		return pushResult(L, v, err)
	}))

	L.SetField(conn, "read_until", L.NewFunction(func(L *lua.LState) int {
		delim := L.CheckString(2)
		if delim == "" {
			L.ArgError(2, "delimiter must not be empty")
			return 0
		}
		v, err := lc.readUntil([]byte(delim))
		return pushResult(L, v, err)
	}))

	L.SetField(conn, "write", L.NewFunction(func(L *lua.LState) int {
		if _, err := lc.Write([]byte(L.CheckString(2))); err != nil {
			L.RaiseError("write: %v", err)
		}
		return 0
	}))

	L.SetField(conn, "sleep", L.NewFunction(func(L *lua.LState) int {
		ms := L.CheckInt(2)
		if ms < 0 {
			L.ArgError(2, "sleep duration must be >= 0")
			return 0
		}
		d := time.Duration(ms) * time.Millisecond
		if d > maxSleep {
			L.ArgError(2, "sleep duration exceeds "+maxSleep.String())
			return 0
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			L.RaiseError("interrupted")
			return 0
		}
		// a sleep is script activity, not client inactivity
		if env.IdleTimeout > 0 {
			_ = lc.SetDeadline(time.Now().Add(env.IdleTimeout))
		}
		return 0
	}))

	L.SetField(conn, "close", L.NewFunction(func(L *lua.LState) int {
		_ = lc.Close()
		return 0
	}))

	L.SetField(conn, "remote", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(lc.RemoteAddr().String()))
		return 1
	}))

	L.SetField(conn, "sni", L.NewFunction(func(L *lua.LState) int {
		name := ""
		if tc := lc.tls(); tc != nil {
			// scripts typically call sni() before any read
			if !tc.ConnectionState().HandshakeComplete {
				_ = tc.HandshakeContext(ctx)
			}
			name = tc.ConnectionState().ServerName
		}
		if name != "" {
			L.Push(lua.LString(name))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	return conn
}

// pushResult returns a value, nil on clean EOF, or raises on failure.
func pushResult(L *lua.LState, v lua.LValue, err error) int {
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	L.Push(v)
	return 1
}
