package smtpserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func TestSMTPServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	srv, err := NewServer(Config{
		Addr:              addr,
		Domain:            "localhost",
		WriteTimeout:      10,
		ReadTimeout:       10,
		MaxMessageBytes:   1024 * 1024,
		MaxRecipients:     50,
		AllowInsecureAuth: true,
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("server error: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("server did not stop")
		}
	})

	c, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
			t.Errorf("Close: %v", err)
		}
	}()

	if err := c.Auth(sasl.NewPlainClient("", "username", "password")); err != nil {
		t.Fatalf("Auth: %v", err)
	}
	if err := c.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := c.Rcpt("recipient@example.net", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	wc, err := c.Data()
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if _, err := fmt.Fprintf(wc, "This is the email body"); err != nil {
		t.Fatalf("Fprintf: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
}

func TestSMTPSServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	cert, err := tlsprovider.GenerateSelfSigned(tlsprovider.SelfSignedOptions{
		DNSNames: []string{"localhost"},
		IPs:      []net.IP{net.ParseIP("127.0.0.1")},
	})
	if err != nil {
		t.Fatalf("GenerateSelfSigned: %v", err)
	}
	serverTLS := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	tlsLn := tls.NewListener(ln, serverTLS)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(Config{
		Addr:              addr,
		Domain:            "localhost",
		WriteTimeout:      10,
		ReadTimeout:       10,
		MaxMessageBytes:   1024 * 1024,
		MaxRecipients:     50,
		AllowInsecureAuth: false,
	}, nil, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.TLSConfig = serverTLS

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(tlsLn)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("server error: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("server did not stop")
		}
	})

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: "127.0.0.1"}

	c, err := smtp.DialTLS(addr, clientTLS)
	if err != nil {
		t.Fatalf("DialTLS: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil && err.Error() != "use of closed network connection" {
			t.Fatalf("Close: %v", err)
		}
	}()

	if err := c.Auth(sasl.NewPlainClient("", "username", "password")); err != nil {
		t.Fatalf("Auth: %v", err)
	}
	if err := c.Mail("sender@example.org", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := c.Rcpt("recipient@example.net", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	wc, err := c.Data()
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if _, err := fmt.Fprintf(wc, "This is the email body"); err != nil {
		t.Fatalf("Fprintf: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
}
