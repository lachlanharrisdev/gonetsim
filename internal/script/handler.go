// Package script owns the handler boundary: resolving a handler spec, running
// sandboxed Lua scripts, and the builtin echo/sink handlers. Everything the
// sandbox exposes to scripts lives in this package, one file per API module,
// so the extension surface is reviewable in a single place.
package script

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/store"
)

// Commenter is the capture annotation seam. pcap.Session implements it.
type Commenter interface {
	Comment(string)
}

// Env is the per-connection environment handed to a handler. Capture and
// Global may be nil when capture is disabled.
type Env struct {
	Logger      *slog.Logger
	Capture     Commenter
	IdleTimeout time.Duration // connection idle timeout, used by conn:sleep
	Global      *store.Store
}

// Handler serves TCP connections and/or UDP datagrams.
type Handler interface {
	ServeTCP(ctx context.Context, conn net.Conn, env Env) error
	ServeUDP(ctx context.Context, data []byte, remote net.Addr, env Env) ([]byte, error)
}

// New resolves a handler spec. Supported forms:
//
//	builtin:echo - echo all received data back
//	builtin:sink - consume and discard all received data
//	lua:<path>   - serve with a Lua script (relative paths resolve against baseDir)
//
// A nil budget gives the handler its own private state budget.
func New(spec, baseDir string, budget *store.Budget) (Handler, error) {
	scheme, value, ok := strings.Cut(spec, ":")
	if !ok {
		return nil, fmt.Errorf("invalid handler %q (expected \"builtin:name\" or \"lua:path\")", spec)
	}

	switch scheme {
	case "builtin":
		switch value {
		case "echo":
			return EchoHandler{}, nil
		case "sink":
			return SinkHandler{}, nil
		default:
			return nil, fmt.Errorf("unknown builtin handler %q (must be echo or sink)", value)
		}
	case "lua":
		path := value
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		return NewLua(path, budget)
	default:
		return nil, fmt.Errorf("unknown handler scheme %q in %q (must be builtin or lua)", scheme, spec)
	}
}

// readError maps a clean EOF to nil.
func readError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
