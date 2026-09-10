package script

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/lachlanharrisdev/gonetsim/internal/store"
)

// installState registers a scoped key/value store as a global table exposing
// get/set/has/delete. Scope depends on the name: conn, handler, or global.
func installState(L *lua.LState, name string, s *store.Store) {
	t := L.NewTable()
	installStateMethods(L, t, s)
	L.SetGlobal(name, t)
}

func installStateMethods(L *lua.LState, t *lua.LTable, s *store.Store) {
	L.SetField(t, "get", L.NewFunction(func(L *lua.LState) int {
		if v, ok := s.Get(L.CheckString(2)); ok {
			L.Push(lua.LString(v))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	L.SetField(t, "set", L.NewFunction(func(L *lua.LState) int {
		if err := s.Set(L.CheckString(2), L.CheckString(3)); err != nil {
			L.Push(lua.LFalse)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(t, "has", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(s.Has(L.CheckString(2))))
		return 1
	}))
	L.SetField(t, "delete", L.NewFunction(func(L *lua.LState) int {
		s.Delete(L.CheckString(2))
		return 0
	}))
}
