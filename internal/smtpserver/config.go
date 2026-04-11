package smtpserver

import (
	"errors"
	"log/slog"

	"github.com/emersion/go-smtp"

	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func (s *Server) Name() string {
	return s.name
}

type Server struct {
	name string
	conf Config
	srv  *smtp.Server
	log  *slog.Logger
}

func NewService(conf Config) service.Service {
	name := "SMTP"
	if conf.TLS != nil {
		name = "SMTPS"
	}
	return &Server{name: name, conf: conf}
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

	// TLS enables SMTPS mode when non-nil.
	TLS *tlsprovider.Config
}

func (c Config) validate() error {
	if c.Addr == "" {
		return errors.New("smtp listen addr is required")
	}

	return nil
}
