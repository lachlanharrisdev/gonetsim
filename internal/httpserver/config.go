package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func (s *Server) Name() string {
	return s.name
}

type Server struct {
	name string
	conf Config
	srv  *http.Server
	log  *slog.Logger
}

func NewService(conf Config, logger *slog.Logger) service.Service {
	name := "HTTP"
	if conf.TLS != nil {
		name = "HTTPS"
	}

	return &Server{name: name, conf: conf.normalize(), log: service.NewPrefixedLogger(logger, name)}
}

type Config struct {
	Addr string

	// enables https if not nil
	TLS *tlsprovider.Config

	// if non-empty, a fixed status code returned for all requests
	// when zero, defaults to 200
	StatusCode int

	// allow switching between modes
	// more info in the docs: https://gonetsim.lachlanharris.dev/reference/http
	Mode string

	// root directory to serve files
	// only used in real mode
	RootDir string
}

// normalize fills in defaults that can't be expressed as zero values.
// Mode defaults to "fake" when empty (for backwards compatibility with
// configs created before the mode/root_dir options existed).
func (c Config) normalize() Config {
	if c.Mode == "" {
		c.Mode = "fake"
	}
	return c
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("http: listen addr is required")
	}
	if c.TLS != nil {
		if err := c.TLS.Validate(); err != nil {
			return err
		}
	}
	switch c.Mode {
	case "", "fake":
		// ok
	case "real":
		if c.RootDir == "" {
			return fmt.Errorf("http: real mode requires root_dir to be set")
		}
		if _, err := os.Stat(c.RootDir); os.IsNotExist(err) {
			return fmt.Errorf("http: root_dir %q does not exist", c.RootDir)
		}
	default:
		return fmt.Errorf("http: mode can only be 'fake' or 'real', was %q", c.Mode)
	}
	return nil
}
