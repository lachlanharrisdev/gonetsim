package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/runutil"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsutil"
	"github.com/spf13/cobra"
)

var (
	serveEnableDNS   bool
	serveEnableHTTP  bool
	serveEnableHTTPS bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the main services (dns + http + https)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := runutil.SignalContext(context.Background())
		defer stop()

		errCh := make(chan error, 3)

		var dnsSrv *dnsserver.Server
		if serveEnableDNS {
			listen, err := parseAddrPort(dnsListen)
			if err != nil {
				return err
			}
			ipv4, err := parseNetipAddr(dnsIPv4)
			if err != nil {
				return err
			}
			ipv6, err := parseOptionalNetipAddr(dnsIPv6)
			if err != nil {
				return err
			}
			dnsSrv, err = dnsserver.New(dnsserver.Config{
				Addr:         listen,
				Net:          dnsNetwork,
				SinkholeIPv4: ipv4,
				SinkholeIPv6: ipv6,
			})
			if err != nil {
				return err
			}
			go func() { errCh <- dnsSrv.ListenAndServe() }()
		}

		var httpSrv *httpserver.Server
		if serveEnableHTTP {
			listen, err := parseAddrPort(httpListen)
			if err != nil {
				return err
			}
			httpSrv, err = httpserver.New(httpserver.Config{Addr: listen, StatusCode: httpStatus}, nil)
			if err != nil {
				return err
			}
			go func() {
				err := httpSrv.ListenAndServe()
				if errors.Is(err, http.ErrServerClosed) {
					err = nil
				}
				errCh <- err
			}()
		}

		var httpsSrv *httpserver.Server
		if serveEnableHTTPS {
			listen, err := parseAddrPort(httpsListen)
			if err != nil {
				return err
			}
			httpsSrv, err = httpserver.New(httpserver.Config{Addr: listen, StatusCode: httpsStatus}, nil)
			if err != nil {
				return err
			}

			go func() {
				if httpsCert != "" || httpsKey != "" {
					err := httpsSrv.ListenAndServeTLS(httpsCert, httpsKey)
					if errors.Is(err, http.ErrServerClosed) {
						err = nil
					}
					errCh <- err
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

				tlsConf := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
				httpsSrv.SetTLSConfig(tlsConf)

				ln, err := net.Listen("tcp", listen)
				if err != nil {
					errCh <- err
					return
				}
				err = httpsSrv.Serve(tls.NewListener(ln, tlsConf))
				if errors.Is(err, http.ErrServerClosed) {
					err = nil
				}
				errCh <- err
			}()
		}

		log.Printf("serve: running (dns=%t http=%t https=%t)", serveEnableDNS, serveEnableHTTP, serveEnableHTTPS)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if dnsSrv != nil {
				_ = dnsSrv.Shutdown(shutdownCtx)
			}
			if httpSrv != nil {
				_ = httpSrv.Shutdown(shutdownCtx)
			}
			if httpsSrv != nil {
				_ = httpsSrv.Shutdown(shutdownCtx)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return nil
			case err := <-errCh:
				if err == nil {
					continue
				}
				log.Printf("serve: server error: %v", err)
				stop()
				return err
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().BoolVar(&serveEnableDNS, "dns", true, "enable DNS")
	serveCmd.Flags().BoolVar(&serveEnableHTTP, "http", true, "enable HTTP")
	serveCmd.Flags().BoolVar(&serveEnableHTTPS, "https", true, "enable HTTPS")

	// Reuse the same flag vars as the individual subcommands so defaults stay consistent.
	serveCmd.Flags().StringVar(&dnsListen, "dns-listen", ":5353", "DNS listen address")
	serveCmd.Flags().StringVar(&dnsNetwork, "dns-network", "udp", "DNS network: udp or tcp")
	serveCmd.Flags().StringVar(&dnsIPv4, "dns-ipv4", "127.0.0.1", "DNS sinkhole IPv4")
	serveCmd.Flags().StringVar(&dnsIPv6, "dns-ipv6", "::1", "DNS sinkhole IPv6 (empty disables)")

	serveCmd.Flags().StringVar(&httpListen, "http-listen", ":8080", "HTTP listen address")
	serveCmd.Flags().IntVar(&httpStatus, "http-status", 200, "HTTP status code")

	serveCmd.Flags().StringVar(&httpsListen, "https-listen", ":8443", "HTTPS listen address")
	serveCmd.Flags().IntVar(&httpsStatus, "https-status", 200, "HTTPS status code")
	serveCmd.Flags().StringVar(&httpsCert, "https-cert", "", "HTTPS TLS cert PEM path (optional)")
	serveCmd.Flags().StringVar(&httpsKey, "https-key", "", "HTTPS TLS key PEM path (optional)")
}
