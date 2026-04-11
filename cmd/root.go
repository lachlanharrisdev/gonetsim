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
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var rootConfigPath string

var rootCmd = &cobra.Command{
	Use:           "gonetsim",
	Short:         "Starts all configured services",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgRes, err := appconfig.LoadOrCreate(rootConfigPath)
		if err != nil {
			return err
		}
		cfg := cfgRes.Config
		if err := cfg.Validate(); err != nil {
			return err
		}

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
		serviceCount := 0

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
			if err := conf.Validate(); err != nil {
				return fmt.Errorf("dns: %w", err)
			}
			manager.Add(dnsserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.HTTP.Enabled {
			listen, err := parseAddrPort(cfg.HTTP.Listen)
			if err != nil {
				return fmt.Errorf("http.listen: %w", err)
			}
			conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTP.Status}
			if err := conf.Validate(); err != nil {
				return fmt.Errorf("http: %w", err)
			}
			manager.Add(httpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.HTTPS.Enabled {
			listen, err := parseAddrPort(cfg.HTTPS.Listen)
			if err != nil {
				return fmt.Errorf("https.listen: %w", err)
			}
			conf := httpserver.Config{
				Addr:       listen,
				StatusCode: cfg.HTTPS.Status,
				TLS:        &tlsprovider.Config{CertFile: cfg.HTTPS.Cert, KeyFile: cfg.HTTPS.Key},
			}
			if err := conf.Validate(); err != nil {
				return fmt.Errorf("https: %w", err)
			}
			manager.Add(httpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.SMTP.Enabled {
			listen, err := parseAddrPort(cfg.SMTP.Addr)
			if err != nil {
				return fmt.Errorf("smtp.addr: %w", err)
			}
			conf := smtpserver.Config{
				Addr:              listen,
				Domain:            cfg.SMTP.Domain,
				WriteTimeout:      cfg.SMTP.WriteTimeout,
				ReadTimeout:       cfg.SMTP.ReadTimeout,
				MaxMessageBytes:   cfg.SMTP.MaxMessageBytes,
				MaxRecipients:     cfg.SMTP.MaxRecipients,
				RequireAuth:       cfg.SMTP.RequireAuth,
				AllowInsecureAuth: cfg.SMTP.AllowInsecureAuth,
			}
			if err := conf.Validate(); err != nil {
				return fmt.Errorf("smtp: %w", err)
			}
			manager.Add(smtpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.SMTPS.Enabled {
			listen, err := parseAddrPort(cfg.SMTPS.Addr)
			if err != nil {
				return fmt.Errorf("smtps.addr: %w", err)
			}
			conf := smtpserver.Config{
				Addr:              listen,
				Domain:            cfg.SMTPS.Domain,
				WriteTimeout:      cfg.SMTPS.WriteTimeout,
				ReadTimeout:       cfg.SMTPS.ReadTimeout,
				MaxMessageBytes:   cfg.SMTPS.MaxMessageBytes,
				MaxRecipients:     cfg.SMTPS.MaxRecipients,
				RequireAuth:       cfg.SMTPS.RequireAuth,
				AllowInsecureAuth: cfg.SMTPS.AllowInsecureAuth,
			}
			conf.TLS = &tlsprovider.Config{CertFile: cfg.SMTPS.Cert, KeyFile: cfg.SMTPS.Key}
			if err := conf.Validate(); err != nil {
				return fmt.Errorf("smtps: %w", err)
			}
			manager.Add(smtpserver.NewService(conf, logger))
			serviceCount++
		}

		if serviceCount == 0 {
			return fmt.Errorf("at least one service must be enabled")
		}

		logger.Info("running", "dns", cfg.DNS.Enabled, "http", cfg.HTTP.Enabled, "https", cfg.HTTPS.Enabled, "smtp", cfg.SMTP.Enabled, "smtps", cfg.SMTPS.Enabled, "services", serviceCount)

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
