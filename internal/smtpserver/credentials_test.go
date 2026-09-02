package smtpserver

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/emersion/go-sasl"
)

type captureHandler struct {
	buf bytes.Buffer
}

func (h *captureHandler) Enabled(ctx context.Context, l slog.Level) bool { return true }
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	// serialize a simple "<message> key=value ..." line
	h.buf.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		h.buf.WriteString(" ")
		h.buf.WriteString(a.Key)
		h.buf.WriteString("=")
		h.buf.WriteString(a.Value.String())
		return true
	})
	h.buf.WriteString("\n")
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler       { return h }

func newCaptureLogger() (*slog.Logger, *captureHandler) {
	h := &captureHandler{}
	return slog.New(h), h
}

func TestSession_CredentialsNotLoggedByDefault(t *testing.T) {
	logger, h := newCaptureLogger()
	s := &Session{logger: logger, requireAuth: true, logCredentials: false}

	srv, err := s.Auth("PLAIN")
	if err != nil {
		t.Fatalf("Auth: %v", err)
	}
	if _, _, err := srv.Next([]byte("\x00bob\x00hunter2")); err != nil {
		t.Fatalf("Next: %v", err)
	}

	out := h.buf.String()
	if !strings.Contains(out, "username=bob") {
		t.Fatalf("expected username to be logged, got %q", out)
	}
	if strings.Contains(out, "hunter2") {
		t.Fatalf("password must not be logged by default, got %q", out)
	}
}

func TestSession_CredentialsLoggedWhenEnabled(t *testing.T) {
	logger, h := newCaptureLogger()
	s := &Session{logger: logger, requireAuth: true, logCredentials: true}

	srv, err := s.Auth(sasl.Plain)
	if err != nil {
		t.Fatalf("Auth: %v", err)
	}
	if _, _, err := srv.Next([]byte("\x00alice\x00s3cret")); err != nil {
		t.Fatalf("Next: %v", err)
	}

	out := h.buf.String()
	if !strings.Contains(out, "username=alice") {
		t.Fatalf("expected username to be logged, got %q", out)
	}
	if !strings.Contains(out, "password=s3cret") {
		t.Fatalf("expected password to be logged when enabled, got %q", out)
	}
}
