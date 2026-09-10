package script

import lua "github.com/yuin/gopher-lua"

// installHTTP reserves the http module for the planned high-level HTTP
// simulation API (status/header/body helpers, session handling). Handlers can
// already simulate HTTP over conn:read_line/write; this stub makes a future
// surface fail loudly instead of silently colliding with scripts.
func installHTTP(L *lua.LState) {
	t := L.NewTable()
	L.SetField(t, "respond", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("http.respond: not implemented yet (simulate HTTP over conn:read_line/write)")
		return 0
	}))
	L.SetGlobal("http", t)
}
