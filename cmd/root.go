package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
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
		configDir := filepath.Dir(cfgRes.Path)
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

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		manager := service.NewManager(cfg.General.ShutdownTimeout, logger)
		serviceCount := 0

		if cfg.DNS.Enabled {
			conf, err := dnsConfig(cfg.DNS)
			if err != nil {
				return err
			}
			manager.Add(dnsserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.HTTP.Enabled {
			conf, err := httpConfig(cfg.HTTP)
			if err != nil {
				return err
			}
			manager.Add(httpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.HTTPS.Enabled {
			conf, err := httpsConfig(cfg.HTTPS, configDir)
			if err != nil {
				return err
			}
			manager.Add(httpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.SMTP.Enabled {
			conf, err := smtpConfig(cfg.SMTP)
			if err != nil {
				return err
			}
			manager.Add(smtpserver.NewService(conf, logger))
			serviceCount++
		}

		if cfg.SMTPS.Enabled {
			conf, err := smtpsConfig(cfg.SMTPS, configDir)
			if err != nil {
				return err
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
