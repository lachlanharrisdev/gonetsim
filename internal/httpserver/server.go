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

type RunOptions struct {
	ShutdownTimeout time.Duration
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

func RunHTTP(ctx context.Context, conf Config, opts RunOptions) error {
	srv, err := NewServer(conf, nil)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", conf.Addr)
	if err != nil {
		return err
	}

	log.Printf("http: listening on %s", conf.Addr)
	return serveUntilDone(ctx, srv, ln, opts)
}

func RunHTTPS(ctx context.Context, conf Config, tlsOpts TLSOptions, opts RunOptions) error {
	if err := tlsOpts.validate(); err != nil {
		return err
	}

	srv, err := NewServer(conf, nil)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", conf.Addr)
	if err != nil {
		return err
	}

	tlsConf, err := buildTLSConfig(tlsOpts)
	if err != nil {
		return err
	}
	srv.TLSConfig = tlsConf

	log.Printf("https: listening on %s", conf.Addr)
	return serveUntilDone(ctx, srv, tls.NewListener(ln, tlsConf), opts)
}

func serveUntilDone(ctx context.Context, srv *http.Server, ln net.Listener, opts RunOptions) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownTimeout := opts.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = 5 * time.Second
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)

		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-time.After(shutdownTimeout):
			return nil
		}
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
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
