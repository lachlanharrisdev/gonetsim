package script

import lua "github.com/yuin/gopher-lua"

// installDNS reserves the dns module for the planned high-level DNS sinkhole
// API. A minimal DNS example handler lives in the repo docs for now.
func installDNS(L *lua.LState) {
	t := L.NewTable()
	L.SetField(t, "answer", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("dns.answer: not implemented yet")
		return 0
	}))
	L.SetGlobal("dns", t)
}
