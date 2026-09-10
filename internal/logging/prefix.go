package logging

import (
	"context"
	"log/slog"
	"strings"
)

// WithPrefix returns a logger whose records carry a [NAME] tag on the front of
// the message, so each listener's output lines up when several run at once.
func WithPrefix(root *slog.Logger, name string) *slog.Logger {
	return slog.New(&prefixHandler{prefix: "[" + strings.ToUpper(name) + "] ", next: root.Handler()})
}

type prefixHandler struct {
	prefix string
	next   slog.Handler
}

func (h *prefixHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *prefixHandler) Handle(ctx context.Context, r slog.Record) error {
	r2 := slog.NewRecord(r.Time, r.Level, h.prefix+r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		r2.AddAttrs(a)
		return true
	})
	return h.next.Handle(ctx, r2)
}

func (h *prefixHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prefixHandler{prefix: h.prefix, next: h.next.WithAttrs(attrs)}
}

func (h *prefixHandler) WithGroup(name string) slog.Handler {
	return &prefixHandler{prefix: h.prefix, next: h.next.WithGroup(name)}
}
