package smtpserver

import (
	"errors"
	"log/slog"

	"github.com/emersion/go-smtp"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
)

func (s *Server) Name() string {
	return s.name
}

type Server struct {
	name    string
	conf    Config
	tlsOpts *TLSOptions
	srv     *smtp.Server
	log     *slog.Logger
}

func NewSMTPService(conf Config) service.Service {
	return &Server{
		name: "SMTP",
		conf: conf,
	}
}
func NewSMTPSService(conf Config, tlsOpts TLSOptions) service.Service {
	return &Server{
		name:    "SMTPS",
		conf:    conf,
		tlsOpts: &tlsOpts,
	}
}

func (s *Server) SetLogger(logger *slog.Logger) {
	s.log = logger
}

type Config struct {
	Addr              string // "localhost:1025"
	Domain            string // "localhost"
	WriteTimeout      int    // 10 seconds
	ReadTimeout       int    // 10 seconds
	MaxMessageBytes   int    // 1024 * 1024
	MaxRecipients     int    // 50
	AllowInsecureAuth bool   // true
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

func (c Config) validate() error {
	if c.Addr == "" {
		return errors.New("smtp listen addr is required")
	}

	return nil
}
