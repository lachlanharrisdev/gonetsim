package httpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"
)

type Server struct {
	conf Config
	http *http.Server
}

func New(conf Config, handler http.Handler) (*Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	if handler == nil {
		handler = Handler{StatusCode: conf.StatusCode}
	}

	s := &http.Server{
		Addr:              conf.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{conf: conf, http: s}, nil
}

func (s *Server) ListenAndServe() error {
	log.Printf("http: listening on %s", s.conf.Addr)
	return s.http.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	log.Printf("https: listening on %s", s.conf.Addr)
	return s.http.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Serve(l net.Listener) error {
	return s.http.Serve(l)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
