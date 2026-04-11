package httpserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func (s *Server) Name() string {
	return s.name
}

type Server struct {
	name    string
	conf    Config
	srv     *http.Server
	log     *slog.Logger
}

func NewService(conf Config) service.Service {
	name := "HTTP"
	if conf.TLS != nil {
		name = "HTTPS"
	}
	return &Server{name: name, conf: conf}
}

func (s *Server) SetLogger(logger *slog.Logger) {
	s.log = logger
}

type Config struct {
	Addr string

	// enables https if not nil
	TLS *tlsprovider.Config

	// if non-empty, a fixed status code returned for all requests
	// when zero, defaults to 200
	StatusCode int
}

func (c Config) validate() error {
	if c.Addr == "" {
		return errors.New("http listen addr is required")
	}
	return nil
}
