package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// prefix handling system, so prefix for services is display whilst still being able to use lmittman/tint
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

func NewPrefixedLogger(rootLogger *slog.Logger, serviceName string) *slog.Logger {
	prefix := fmt.Sprintf("%-5s ", strings.ToUpper(serviceName))
	return slog.New(&prefixHandler{prefix: prefix, next: rootLogger.Handler()})
}
