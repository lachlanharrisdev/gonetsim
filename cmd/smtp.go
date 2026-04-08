package cmd

import (
	"context"
	"log/slog"
	"time"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var (
	smtpAddr              string
	smtpDomain            string
	smtpWriteTimeout      int
	smtpReadTimeout       int
	smtpMaxMessageBytes   int
	smtpMaxRecipients     int
	smtpAllowInsecureAuth bool
)

var smtpCmd = &cobra.Command{
	Use:   "smtp",
	Short: "Run an SMTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(smtpAddr)
		if err != nil {
			return err
		}

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()

		logger, err := observability.NewLogger(appconfig.Default().Logging)
		if err != nil {
			return err
		}
		slog.SetDefault(logger)
		manager := service.NewManager(5*time.Second, logger)

		return manager.RunSingleService(ctx,
			smtpserver.NewSMTPService(
				smtpserver.Config{
					Addr:              listen,
					Domain:            smtpDomain,
					WriteTimeout:      smtpWriteTimeout,
					ReadTimeout:       smtpReadTimeout,
					MaxMessageBytes:   smtpMaxMessageBytes,
					MaxRecipients:     smtpMaxRecipients,
					AllowInsecureAuth: smtpAllowInsecureAuth,
				},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(smtpCmd)

	smtpCmd.Flags().StringVar(&smtpAddr, "listen", ":1025", "listen address")
	smtpCmd.Flags().StringVar(&smtpDomain, "domain", "localhost", "SMTP server domain")
	smtpCmd.Flags().IntVar(&smtpWriteTimeout, "write-timeout", 10, "write timeout in seconds")
	smtpCmd.Flags().IntVar(&smtpReadTimeout, "read-timeout", 10, "read timeout in seconds")
	smtpCmd.Flags().IntVar(&smtpMaxMessageBytes, "max-message-bytes", 1024*1024, "max message size in bytes")
	smtpCmd.Flags().IntVar(&smtpMaxRecipients, "max-recipients", 50, "maximum number of recipients per message")
	smtpCmd.Flags().BoolVar(&smtpAllowInsecureAuth, "allow-insecure-auth", true, "allow authentication without TLS")
}
