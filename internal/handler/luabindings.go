package handler

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

func registerState(L *lua.LState, name string, store *state.Store) {
	t := L.NewTable()
	registerStateMethods(L, t, store)
	L.SetGlobal(name, t)
}

func registerStateMethods(L *lua.LState, t *lua.LTable, store *state.Store) {
	L.SetField(t, "get", L.NewFunction(func(L *lua.LState) int {
		if v, ok := store.Get(L.CheckString(2)); ok {
			L.Push(lua.LString(v))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	L.SetField(t, "set", L.NewFunction(func(L *lua.LState) int {
		if err := store.Set(L.CheckString(2), L.CheckString(3)); err != nil {
			L.Push(lua.LFalse)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(t, "has", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(store.Has(L.CheckString(2))))
		return 1
	}))
	L.SetField(t, "delete", L.NewFunction(func(L *lua.LState) int {
		store.Delete(L.CheckString(2))
		return 0
	}))
}

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

func registerCapture(L *lua.LState, ses *capture.Session) {
	capture := L.NewTable()
	L.SetField(capture, "comment", L.NewFunction(func(L *lua.LState) int {
		ses.Comment(L.CheckString(2))
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

func registerConn(L *lua.LState, lc *luaConn, ctx context.Context, env Env, connState *state.Store) *lua.LTable {
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

	L.SetField(conn, "local", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(lc.LocalAddr().String()))
		return 1
	}))

	L.SetField(conn, "remote_ip", L.NewFunction(func(L *lua.LState) int {
		return pushTCPIP(L, lc.RemoteAddr())
	}))

	L.SetField(conn, "remote_port", L.NewFunction(func(L *lua.LState) int {
		return pushTCPPort(L, lc.RemoteAddr())
	}))

	L.SetField(conn, "local_port", L.NewFunction(func(L *lua.LState) int {
		return pushTCPPort(L, lc.LocalAddr())
	}))

	L.SetField(conn, "sni", L.NewFunction(func(L *lua.LState) int {
		if st, ok := lc.handshake(ctx); ok && st.ServerName != "" {
			L.Push(lua.LString(st.ServerName))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	L.SetField(conn, "tls", L.NewFunction(func(L *lua.LState) int {
		st, ok := lc.handshake(ctx)
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		info := L.NewTable()
		L.SetField(info, "version", lua.LString(tls.VersionName(st.Version)))
		L.SetField(info, "cipher", lua.LString(tls.CipherSuiteName(st.CipherSuite)))
		L.Push(info)
		return 1
	}))

	registerStateMethods(L, conn, connState)

	return conn
}

func pushResult(L *lua.LState, v lua.LValue, err error) int {
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	L.Push(v)
	return 1
}

func pushTCPIP(L *lua.LState, addr net.Addr) int {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		L.Push(lua.LString(tcp.IP.String()))
	} else {
		L.Push(lua.LNil)
	}
	return 1
}

func pushTCPPort(L *lua.LState, addr net.Addr) int {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		L.Push(lua.LNumber(tcp.Port))
	} else {
		L.Push(lua.LNil)
	}
	return 1
}
