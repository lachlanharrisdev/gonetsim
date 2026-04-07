package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var rootConfigPath string

var rootCmd = &cobra.Command{
	Use:          "gonetsim",
	Short:        "Network service simulator (dns + http + https)",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
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

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		errCh := make(chan error, 3)
		var wg sync.WaitGroup

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
			conf := dnsserver.Config{Addr: listen, Net: cfg.DNS.Network, SinkholeIPv4: ipv4, SinkholeIPv6: ipv6}
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- dnsserver.Run(runCtx, conf, dnsserver.RunOptions{ShutdownTimeout: cfg.General.ShutdownTimeout})
			}()
		}

		if cfg.HTTP.Enabled {
			listen, err := parseAddrPort(cfg.HTTP.Listen)
			if err != nil {
				return fmt.Errorf("http.listen: %w", err)
			}
			conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTP.Status}
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- httpserver.RunHTTP(runCtx, conf, httpserver.RunOptions{ShutdownTimeout: cfg.General.ShutdownTimeout})
			}()
		}

		if cfg.HTTPS.Enabled {
			listen, err := parseAddrPort(cfg.HTTPS.Listen)
			if err != nil {
				return fmt.Errorf("https.listen: %w", err)
			}
			conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTPS.Status}
			tlsOpts := httpserver.TLSOptions{CertFile: cfg.HTTPS.Cert, KeyFile: cfg.HTTPS.Key}
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- httpserver.RunHTTPS(runCtx, conf, tlsOpts, httpserver.RunOptions{ShutdownTimeout: cfg.General.ShutdownTimeout})
			}()
		}

		log.Printf("root: running (dns=%t http=%t https=%t)", cfg.DNS.Enabled, cfg.HTTP.Enabled, cfg.HTTPS.Enabled)

		go func() {
			wg.Wait()
			close(errCh)
		}()

		for {
			select {
			case <-ctx.Done():
				cancel()
				return nil
			case err, ok := <-errCh:
				if !ok {
					return nil
				}
				if err == nil {
					continue
				}
				log.Printf("root: server error: %v", err)
				cancel()
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
