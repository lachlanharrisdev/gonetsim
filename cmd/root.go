package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var rootConfigPath string

var rootCmd = &cobra.Command{
	Use:           "gonetsim",
	Short:         "Network service simulator (dns + http + https)",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgRes, err := appconfig.LoadOrCreate(rootConfigPath)
		if err != nil {
			return err
		}
		cfg := cfgRes.Config

		logger, err := observability.NewLogger(cfg.Logging)
		if err != nil {
			return err
		}
		slog.SetDefault(logger)
		if cfgRes.Created {
			logger.Info("config created", "path", cfgRes.Path)
		}
		logger.Info("config loaded", "path", cfgRes.Path)

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		manager := service.NewManager(cfg.General.ShutdownTimeout, logger)

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
			conf := dnsserver.Config{
				Addr:           listen,
				Net:            cfg.DNS.Network,
				SinkholeIPv4:   ipv4,
				SinkholeIPv6:   ipv6,
				SinkholeDomain: cfg.DNS.Domain,
				SinkholeTXT:    cfg.DNS.TXT,
				TTL:            cfg.DNS.TTL,
				Compress:       cfg.DNS.Compress,
			}
			manager.Add(dnsserver.NewService(conf))
		}

		if cfg.HTTP.Enabled {
			listen, err := parseAddrPort(cfg.HTTP.Listen)
			if err != nil {
				return fmt.Errorf("http.listen: %w", err)
			}
			conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTP.Status}
			manager.Add(httpserver.NewHTTPService(conf))
		}

		if cfg.HTTPS.Enabled {
			listen, err := parseAddrPort(cfg.HTTPS.Listen)
			if err != nil {
				return fmt.Errorf("https.listen: %w", err)
			}
			conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTPS.Status}
			tlsOpts := httpserver.TLSOptions{CertFile: cfg.HTTPS.Cert, KeyFile: cfg.HTTPS.Key}
			manager.Add(httpserver.NewHTTPSService(conf, tlsOpts))
		}

		logger.Info("running", "dns", cfg.DNS.Enabled, "http", cfg.HTTP.Enabled, "https", cfg.HTTPS.Enabled)

		return manager.RunAll(runCtx)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal error", "err", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootConfigPath, "config", "", "path to config TOML file (optional)")
}
