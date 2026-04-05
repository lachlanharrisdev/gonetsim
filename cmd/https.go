package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/runutil"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsutil"
	"github.com/spf13/cobra"
)

var (
	httpsListen string
	httpsStatus int
	httpsCert   string
	httpsKey    string
)

var httpsCmd = &cobra.Command{
	Use:   "https",
	Short: "Run an HTTPS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(httpsListen)
		if err != nil {
			return err
		}

		srv, err := httpserver.New(httpserver.Config{Addr: listen, StatusCode: httpsStatus}, nil)
		if err != nil {
			return err
		}

		ctx, stop := runutil.SignalContext(context.Background())
		defer stop()

		errCh := make(chan error, 1)
		go func() {
			if httpsCert != "" || httpsKey != "" {
				errCh <- srv.ListenAndServeTLS(httpsCert, httpsKey)
				return
			}

			cert, err := tlsutil.GenerateSelfSigned(tlsutil.SelfSignedOptions{
				DNSNames: []string{"localhost"},
				IPs:      []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
			})
			if err != nil {
				errCh <- err
				return
			}

			tlsConf := &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			}
			srv.SetTLSConfig(tlsConf)

			ln, err := net.Listen("tcp", listen)
			if err != nil {
				errCh <- err
				return
			}
			errCh <- srv.Serve(tls.NewListener(ln, tlsConf))
		}()

		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
			return nil
		case err := <-errCh:
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Printf("https: server error: %v", err)
			return err
		}
	},
}

func init() {
	rootCmd.AddCommand(httpsCmd)

	httpsCmd.Flags().StringVar(&httpsListen, "listen", ":8443", "listen address")
	httpsCmd.Flags().IntVar(&httpsStatus, "status", 200, "status code to return for all requests")
	httpsCmd.Flags().StringVar(&httpsCert, "cert", "", "path to TLS cert PEM (optional; defaults to ephemeral self-signed)")
	httpsCmd.Flags().StringVar(&httpsKey, "key", "", "path to TLS key PEM (optional; defaults to ephemeral self-signed)")
}
