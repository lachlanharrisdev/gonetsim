package httpserver

import (
	"errors"
	"net/http"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
)

func (s *Server) Name() string {
	return s.name
}

type Server struct {
	name    string
	conf    Config
	tlsOpts *TLSOptions
	srv     *http.Server
}

func NewHTTPService(conf Config) service.Service {
	return &Server{
		name: "HTTP",
		conf: conf,
	}
}

func NewHTTPSService(conf Config, tlsOpts TLSOptions) service.Service {
	return &Server{
		name:    "HTTPS",
		conf:    conf,
		tlsOpts: &tlsOpts,
	}
}

type Config struct {
	Addr string

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

type TLSOptions struct {
	CertFile string
	KeyFile  string
}

func (o TLSOptions) isProvided() bool {
	return o.CertFile != "" || o.KeyFile != ""
}

func (o TLSOptions) validate() error {
	if (o.CertFile == "") != (o.KeyFile == "") {
		return errors.New("cert and key must be set together")
	}
	return nil
}
