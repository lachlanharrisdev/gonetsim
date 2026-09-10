package script

import lua "github.com/yuin/gopher-lua"

// installCapture registers the capture table. Scripts annotate interesting
// packets with capture:comment("...") which becomes a packet comment in
// Wireshark. Safe to call with a nil Commenter (capture disabled): the
// function silently no-ops.
func installCapture(L *lua.LState, c Commenter) {
	t := L.NewTable()
	L.SetField(t, "comment", L.NewFunction(func(L *lua.LState) int {
		if c != nil {
			c.Comment(L.CheckString(2))
		}
		return 0
	}))
	L.SetGlobal("capture", t)
}
