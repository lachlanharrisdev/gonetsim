package handler

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"

	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

const (
	maxReadLen = 1 << 20 // caps read_line/read_until, 1 MiB
	maxSleep   = time.Hour

	tcpEntry = "handle"        // handle(conn)
	udpEntry = "handle_packet" // handle_packet(data) -> string | nil
)

type LuaHandler struct {
	path         string
	proto        *lua.FunctionProto
	budget       *state.Budget
	handlerState *state.Store
}

func NewLua(path string, budget *state.Budget) (*LuaHandler, error) {
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

	return &LuaHandler{
		path:         path,
		proto:        proto,
		budget:       budget,
		handlerState: state.NewStore(budget),
	}, nil
}

func (h *LuaHandler) HandleTCP(ctx context.Context, conn net.Conn, env Env) error {
	L := h.newState(env)
	defer L.Close()

	lc := newLuaConn(conn)
	connState := state.NewStore(h.budget)
	if _, err := h.run(L, tcpEntry, 0, registerConn(L, lc, ctx, env, connState)); err != nil {
		return fmt.Errorf("lua %s: %w", h.path, err)
	}
	return nil
}

func (h *LuaHandler) HandleUDP(_ context.Context, data []byte, remote net.Addr, env Env) ([]byte, error) {
	L := h.newState(env)
	defer L.Close()

	peer := L.NewTable()
	L.SetField(peer, "addr", lua.LString(remote.String()))
	if udp, ok := remote.(*net.UDPAddr); ok {
		L.SetField(peer, "ip", lua.LString(udp.IP.String()))
		L.SetField(peer, "port", lua.LNumber(udp.Port))
	}

	ret, err := h.run(L, udpEntry, 1, lua.LString(data), peer)
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

	global := env.Global
	if global == nil {
		global = state.NewStore(h.budget)
	}
	registerState(L, "global", global)
	registerState(L, "handler", h.handlerState)
	return L
}
