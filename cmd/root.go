package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/runutil"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsutil"
	"github.com/spf13/cobra"
)

var rootConfigPath string

var rootCmd = &cobra.Command{
	Use: "gonetsim",
	Short: "Network service simulator (dns + http + https)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgRes, err := appconfig.LoadOrCreate(rootConfigPath)
		if err != nil {
			return err
		}
		if cfgRes.Created {
			log.Printf("config: created default config at %s", cfgRes.Path)
		}
		log.Printf("config: using %s", cfgRes.Path)
		cfg := cfgRes.Config

		ctx, stop := runutil.SignalContext(context.Background())
		defer stop()

		errCh := make(chan error, 3)

		var dnsSrv *dnsserver.Server
		if cfg.DNS.Enabled {
			listen, err := parseAddrPort(cfg.DNS.Listen)
			if err != nil {
				return fmt.Errorf("dns.listen: %w", err)
			}
			ipv4, err := parseNetipAddr(cfg.DNS.IPv4)
			if err != nil {
				return fmt.Errorf("dns.ipv4: %w", err)
			}
			ipv6, err := parseOptionalNetipAddr(cfg.DNS.IPv6)
			if err != nil {
				return fmt.Errorf("dns.ipv6: %w", err)
			}
			dnsSrv, err = dnsserver.New(dnsserver.Config{
				Addr:         listen,
				Net:          cfg.DNS.Network,
				SinkholeIPv4: ipv4,
				SinkholeIPv6: ipv6,
			})
			if err != nil {
				return err
			}
			go func() { errCh <- dnsSrv.ListenAndServe() }()
		}

		var httpSrv *httpserver.Server
		if cfg.HTTP.Enabled {
			listen, err := parseAddrPort(cfg.HTTP.Listen)
			if err != nil {
				return fmt.Errorf("http.listen: %w", err)
			}
			httpSrv, err = httpserver.New(httpserver.Config{Addr: listen, StatusCode: cfg.HTTP.Status}, nil)
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
		if cfg.HTTPS.Enabled {
			listen, err := parseAddrPort(cfg.HTTPS.Listen)
			if err != nil {
				return fmt.Errorf("https.listen: %w", err)
			}
			httpsSrv, err = httpserver.New(httpserver.Config{Addr: listen, StatusCode: cfg.HTTPS.Status}, nil)
			if err != nil {
				return err
			}

			go func() {
				// User-provided cert/key.
				if cfg.HTTPS.Cert != "" || cfg.HTTPS.Key != "" {
					err := httpsSrv.ListenAndServeTLS(cfg.HTTPS.Cert, cfg.HTTPS.Key)
					if errors.Is(err, http.ErrServerClosed) {
						err = nil
					}
					errCh <- err
					return
				}

				// Ephemeral self-signed cert.
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

		log.Printf("root: running (dns=%t http=%t https=%t)", cfg.DNS.Enabled, cfg.HTTP.Enabled, cfg.HTTPS.Enabled)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.General.ShutdownTimeout)
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
				log.Printf("root: server error: %v", err)
				stop()
				return err
			}
		}
	},
}

func exitError(msg interface{}) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		exitError(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootConfigPath, "config", "", "path to config TOML file (optional)")
}
