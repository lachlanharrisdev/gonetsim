package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	"github.com/lachlanharrisdev/gonetsim/internal/netx"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
)

type Server struct {
	name string
	conf Config
	srv  *http.Server
	log  *slog.Logger
	run  *capture.Run
}

func NewService(conf Config, logger *slog.Logger, run *capture.Run) service.Service {
	name := "HTTP"
	if conf.TLS != nil {
		name = "HTTPS"
	}
	if !conf.Capture {
		run = nil
	}

	return &Server{name: name, conf: conf.normalize(), log: service.NewPrefixedLogger(logger, name), run: run}
}

func (s *Server) Name() string {
	return s.name
}

func NewServer(conf Config, handler http.Handler, logger *slog.Logger) (*http.Server, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	conf = conf.normalize()

	if handler == nil {
		if conf.Mode == "real" {
			handler = RealHandler{StatusCode: conf.StatusCode, RootDir: conf.RootDir, Logger: logger}
		} else {
			handler = FakeHandler{StatusCode: conf.StatusCode, Logger: logger}
		}
	}

	srv := &http.Server{
		Addr:              conf.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv, nil
}

func (s *Server) Start(ctx context.Context) error {
	logger := s.log

	srv, err := NewServer(s.conf, nil, logger)
	if err != nil {
		return err
	}
	s.srv = srv

	var tlsConf *tls.Config
	if s.conf.TLS != nil {
		tlsConf, err = s.conf.TLS.TLSConfig()
		if err != nil {
			return err
		}
		srv.TLSConfig = tlsConf
	}
	iface, err := s.run.NewInterface("gonetsim " + strings.ToLower(s.name) + " tcp")
	if err != nil {
		return err
	}
	ln, err := netx.ListenTCP(s.conf.Addr, s.run, iface, tlsConf)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	logger.Info("listening", "on", s.conf.Addr, "mode", s.conf.Mode)
	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}
