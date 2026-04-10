package smtpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

type errorLogger struct {
	logger *slog.Logger
}

func (l errorLogger) Printf(format string, v ...interface{}) {
	if l.logger == nil {
		slog.Error("smtp error", "msg", fmt.Sprintf(format, v...))
		return
	}
	l.logger.Error("smtp error", "msg", fmt.Sprintf(format, v...))
}

func (l errorLogger) Println(v ...interface{}) {
	if l.logger == nil {
		slog.Error("smtp error", "msg", fmt.Sprintln(v...))
		return
	}
	l.logger.Error("smtp error", "msg", fmt.Sprintln(v...))
}

func NewServer(conf Config, tlsOpts *TLSOptions, logger *slog.Logger) (*smtp.Server, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	backend := &Backend{logger: logger}

	// Use smtp.NewServer to ensure internal fields like 'done' channel are initialized
	srv := smtp.NewServer(backend)
	srv.Addr = conf.Addr
	srv.Domain = conf.Domain
	srv.AllowInsecureAuth = conf.AllowInsecureAuth
	srv.MaxMessageBytes = int64(conf.MaxMessageBytes)
	srv.MaxRecipients = conf.MaxRecipients
	srv.ReadTimeout = time.Duration(conf.ReadTimeout) * time.Second
	srv.WriteTimeout = time.Duration(conf.WriteTimeout) * time.Second

	if tlsOpts != nil {
		if err := tlsOpts.validate(); err != nil {
			return nil, err
		}
		tlsConf, err := buildTLSConfig(*tlsOpts)
		if err != nil {
			return nil, err
		}
		srv.TLSConfig = tlsConf
	}
	srv.ErrorLog = errorLogger{logger: logger}

	return srv, nil
}

func (s *Server) Start(ctx context.Context) error {
	logger := s.log
	if logger == nil {
		logger = slog.Default().With("service", s.Name())
	}

	srv, err := NewServer(s.conf, s.tlsOpts, logger)
	if err != nil {
		return err
	}
	s.srv = srv

	logger.Info("listening", "on", s.conf.Addr)
	if s.tlsOpts != nil {
		if err := srv.ListenAndServeTLS(); err != nil {
			return err
		}
	} else {
		if err := srv.ListenAndServe(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	err := s.srv.Shutdown(ctx)
	if errors.Is(err, smtp.ErrServerClosed) {
		return nil
	}
	return err
}

// Backend implements SMTP server methods.
type Backend struct {
	logger *slog.Logger
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remoteAddr := c.Conn().RemoteAddr().String()
	bkd.logger.Info("session started", "remote_addr", remoteAddr)
	return &Session{logger: bkd.logger, remoteAddr: remoteAddr}, nil
}

// Session represents an SMTP session.
type Session struct {
	logger     *slog.Logger
	remoteAddr string
	auth       bool
	username   string
	from       string
	recipients []string
}

// AuthMechanisms returns available authentication mechanisms.
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth handles authentication.
func (s *Session) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		s.username = username
		s.logger.Info("authentication attempt",
			"remote_addr", s.remoteAddr,
			"mechanism", mech,
			"username", username,
		)
		s.auth = true
		return nil
	}), nil
}

// Mail handles MAIL FROM command.
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	s.from = from
	s.recipients = []string{} // Reset recipients for new message
	s.logger.Info("mail from",
		"remote_addr", s.remoteAddr,
		"username", s.username,
		"from", from,
	)
	return nil
}

// Rcpt handles RCPT TO command.
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	s.recipients = append(s.recipients, to)
	s.logger.Info("rcpt to",
		"remote_addr", s.remoteAddr,
		"username", s.username,
		"from", s.from,
		"to", to,
		"recipient_count", len(s.recipients),
	)
	return nil
}

// Data handles the message data.
func (s *Session) Data(r io.Reader) error {
	if !s.auth {
		return smtp.ErrAuthRequired
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.logger.Info("message data received",
		"remote_addr", s.remoteAddr,
		"username", s.username,
		"from", s.from,
		"recipient_count", len(s.recipients),
		"recipients", s.recipients,
		"bytes", len(b),
	)
	return nil
}

// Reset clears the session state.
func (s *Session) Reset() {
	s.auth = false
	s.username = ""
	s.from = ""
	s.recipients = []string{}
	s.logger.Info("session reset",
		"remote_addr", s.remoteAddr,
	)
}

func (s *Session) Logout() error {
	s.logger.Info("session closed",
		"remote_addr", s.remoteAddr,
		"username", s.username,
	)
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
