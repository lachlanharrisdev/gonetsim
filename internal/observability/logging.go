package observability

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"

	"github.com/lachlanharrisdev/gonetsim/internal/config"
)

func NewLogger(cfg config.LoggingConfig) (*slog.Logger, error) {
	level := parseLevel(cfg.Level)
	if strings.ToLower(strings.TrimSpace(cfg.LogFormat)) == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})), nil
	}

	w := os.Stderr
	noColor := !isatty.IsTerminal(w.Fd())

	return slog.New(tint.NewHandler(colorable.NewColorable(w), &tint.Options{
		Level:      level,
		TimeFormat: time.TimeOnly,
		NoColor:    noColor,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Value.Kind() == slog.KindAny {
				if _, ok := a.Value.Any().(error); ok {
					return tint.Attr(9, a)
				}
			}
			return a
		},
	})), nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
