package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func NewServer(conf Config, handler http.Handler, logger *slog.Logger) (*http.Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	if handler == nil {
		handler = FakeHandler{StatusCode: conf.StatusCode, Logger: logger}
	}

	srv := &http.Server{
		Addr:              conf.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
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

	ln, err := net.Listen("tcp", s.conf.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	if s.conf.TLS != nil {
		tlsConf, err := s.conf.TLS.TLSConfig()
		if err != nil {
			return err
		}
		srv.TLSConfig = tlsConf
		ln = tls.NewListener(ln, tlsConf)
	}

	logger.Info("listening", "on", s.conf.Addr)
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
