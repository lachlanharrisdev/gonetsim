package smtpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	smtputils "github.com/lachlanharrisdev/gonetsim/internal/smtpserver/utils"
)

type errorLogger struct {
	logger *slog.Logger
}

func (l errorLogger) Printf(format string, v ...interface{}) {
	l.logger.Error("smtp error", "msg", fmt.Sprintf(format, v...))
}

func (l errorLogger) Println(v ...interface{}) {
	l.logger.Error("smtp error", "msg", fmt.Sprintln(v...))
}

func NewServer(conf Config, logger *slog.Logger) (*smtp.Server, error) {
	backend := &Backend{logger: logger, requireAuth: conf.RequireAuth}

	// Use smtp.NewServer to ensure internal fields like 'done' channel are initialized
	srv := smtp.NewServer(backend)
	srv.Addr = conf.Addr
	srv.Domain = conf.Domain
	srv.AllowInsecureAuth = conf.AllowInsecureAuth
	srv.MaxMessageBytes = int64(conf.MaxMessageBytes)
	srv.MaxRecipients = conf.MaxRecipients
	srv.ReadTimeout = time.Duration(conf.ReadTimeout) * time.Second
	srv.WriteTimeout = time.Duration(conf.WriteTimeout) * time.Second

	if conf.TLS != nil {
		tlsConf, err := conf.TLS.TLSConfig()
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

	srv, err := NewServer(s.conf, logger)
	if err != nil {
		return err
	}
	s.srv = srv

	logger.Info("listening", "on", s.conf.Addr)
	if s.conf.TLS != nil {
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
	logger      *slog.Logger
	requireAuth bool
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remoteAddr := c.Conn().RemoteAddr().String()
	bkd.logger.Info("session started", "remote_addr", remoteAddr)
	return &Session{logger: bkd.logger, remoteAddr: remoteAddr, requireAuth: bkd.requireAuth}, nil
}

// Session represents an SMTP session.
type Session struct {
	logger      *slog.Logger
	remoteAddr  string
	requireAuth bool
	auth        bool
	username    string
	from        string
	recipients  []string
}

// AuthMechanisms returns available authentication mechanisms.
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain, sasl.Anonymous, sasl.Login}
}

// Auth handles authentication.
func (s *Session) Auth(mech string) (sasl.Server, error) {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			s.username = username
			s.logger.Info("authentication attempt",
				"remote_addr", s.remoteAddr,
				"mechanism", mech,
			)
			return nil
		}), nil
	case sasl.Anonymous:
		return sasl.NewAnonymousServer(func(trace string) error {
			s.username = trace
			s.logger.Info("authentication attempt",
				"remote_addr", s.remoteAddr,
				"mechanism", mech,
			)
			return nil
		}), nil
	case sasl.Login:
		return smtputils.NewLoginServer(func(username, password string) error {
			s.username = username
			return nil
		}), nil
	default: // default to ANONYMOUS to hopefully satisfy clients
		s.logger.Warn("unsupported authentication mechanism; defaulting to ANONYMOUS", "remote_addr", s.remoteAddr, "mechanism", mech)
		return sasl.NewAnonymousServer(func(trace string) error {
			s.username = trace
			s.logger.Info("authentication attempt",
				"remote_addr", s.remoteAddr,
				"mechanism", mech,
			)
			return nil
		}), nil
	}
}

// Mail handles MAIL FROM command.
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if s.requireAuth && !s.auth {
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
	if s.requireAuth && !s.auth {
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
	if s.requireAuth && !s.auth {
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
