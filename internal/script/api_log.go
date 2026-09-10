package script

import (
	"context"
	"log/slog"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installLog registers the log table (log:info/warn/error) and reroutes print
// to the listener's logger.
func installLog(L *lua.LState, logger *slog.Logger) {
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

func luaStrings(L *lua.LState) string {
	parts := make([]string, L.GetTop())
	for i := range parts {
		parts[i] = L.ToString(i + 1)
	}
	return strings.Join(parts, " ")
}
