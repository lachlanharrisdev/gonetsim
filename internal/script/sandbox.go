package script

import lua "github.com/yuin/gopher-lua"

// openLibs opens a curated subset of the Lua standard library: base, string,
// table and math. Everything else (io, os, package, coroutine, debug) is never
// opened, and the filesystem/reflection escape hatches the base library
// otherwise exposes are explicitly removed. Scripts cannot read files, run
// code they did not define, or reach into the host.
func openLibs(L *lua.LState) {
	lua.OpenBase(L)
	lua.OpenString(L)
	lua.OpenTable(L)
	lua.OpenMath(L)

	for _, name := range []string{
		"dofile", "loadfile", "load", "loadstring", "require",
		"collectgarbage", "newproxy",
		"rawget", "rawset", "rawequal", "rawlen",
		"getfenv", "setfenv", "module",
	} {
		L.SetGlobal(name, lua.LNil)
	}
}
