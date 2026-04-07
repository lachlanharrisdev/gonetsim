package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func NewServer(conf Config, handler http.Handler) (*http.Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	if handler == nil {
		handler = FakeHandler{StatusCode: conf.StatusCode}
	}

	srv := &http.Server{
		Addr:              conf.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.tlsOpts != nil {
		if err := s.tlsOpts.validate(); err != nil {
			return err
		}
	}

	srv, err := NewServer(s.conf, nil)
	if err != nil {
		return err
	}
	s.srv = srv

	ln, err := net.Listen("tcp", s.conf.Addr)
	if err != nil {
		return err
	}

	if s.tlsOpts != nil {
		tlsConf, err := buildTLSConfig(*s.tlsOpts)
		if err != nil {
			return err
		}
		srv.TLSConfig = tlsConf
		ln = tls.NewListener(ln, tlsConf)
	}

	log.Printf("%s: listening on %s", s.name, s.conf.Addr)
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

func buildTLSConfig(tlsOpts TLSOptions) (*tls.Config, error) {
	if tlsOpts.isProvided() {
		cert, err := tls.LoadX509KeyPair(tlsOpts.CertFile, tlsOpts.KeyFile)
		if err != nil {
			return nil, err
		}
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
	}

	cert, err := tlsprovider.GenerateSelfSigned(tlsprovider.SelfSignedOptions{DNSNames: []string{"localhost"}})
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
}
