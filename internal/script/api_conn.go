package script

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/lachlanharrisdev/gonetsim/internal/store"
)

// installConn registers the conn object for TCP handlers. installStateMethods
// is mixed in so a connection carries its own scoped key/value state.
func installConn(L *lua.LState, lc *luaConn, ctx context.Context, env Env, connState *store.Store) *lua.LTable {
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

	installStateMethods(L, conn, connState)

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
