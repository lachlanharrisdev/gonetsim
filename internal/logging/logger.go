// Package logging builds the process-wide slog logger. Human output is
// message-only - no key=value fields, ever - and colourised on a TTY by charm;
// the same events are emitted machine-readable with --log-format json.
package logging

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
)

// New returns the root logger. level is one of debug, info, warn, error
// (default info); format is "text" (default) or "json".
func New(level, format string) (*slog.Logger, error) {
	opts := log.Options{
		Level:           log.Level(parseLevel(level)),
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		lg := log.NewWithOptions(os.Stderr, opts)
		lg.SetStyles(textStyles())
		return slog.New(lg), nil
	case "json":
		opts.Formatter = log.JSONFormatter
		return slog.New(log.NewWithOptions(os.Stderr, opts)), nil
	default:
		return nil, errors.New("log format must be \"text\" or \"json\"")
	}
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

// textStyles uses three-letter level tags instead of the default full names so
// lines stay compact. Colours only ever render on a real terminal; charm
// strips ANSI when the output is piped or on a bare Windows console.
func textStyles() *log.Styles {
	s := log.DefaultStyles()
	s.Levels = map[log.Level]lipgloss.Style{
		log.DebugLevel: tagStyle("DBG", "63"),
		log.InfoLevel:  tagStyle("INF", "86"),
		log.WarnLevel:  tagStyle("WRN", "192"),
		log.ErrorLevel: tagStyle("ERR", "204"),
	}
	return s
}

func tagStyle(tag, color string) lipgloss.Style {
	return lipgloss.NewStyle().SetString(tag).Bold(true).Width(3).Foreground(lipgloss.Color(color))
}
