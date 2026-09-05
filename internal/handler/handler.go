package handler

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

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

type Env struct {
	Logger      *slog.Logger
	Capture     *capture.Writer
	IdleTimeout time.Duration // connection idle timeout, used by conn:sleep
	Global      *state.Store
}

// Handler processes network traffic for a listener.
type Handler interface {
	// HandleTCP serves a single accepted connection until it is closed.
	HandleTCP(ctx context.Context, conn net.Conn, env Env) error

	// HandleUDP processes a single datagram and returns an optional reply.
	HandleUDP(ctx context.Context, data []byte, remote net.Addr, env Env) ([]byte, error)
}

// New resolves a handler spec. Supported forms:
//
//	builtin:echo - echo all received data back
//	builtin:sink - consume and discard all received data
//	lua:<path>   - serve with a Lua script (relative paths resolve against baseDir)
//
// A nil budget gives the handler its own private state budget
func New(spec string, baseDir string, budget *state.Budget) (Handler, error) {
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
