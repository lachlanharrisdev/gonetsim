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
	smtpsAddr              string
	smtpsDomain            string
	smtpsWriteTimeout      int
	smtpsReadTimeout       int
	smtpsMaxMessageBytes   int
	smtpsMaxRecipients     int
	smtpsAllowInsecureAuth bool
	smtpsCert              string
	smtpsKey               string
)

var smtpsCmd = &cobra.Command{
	Use:   "smtps",
	Short: "Run an SMTPS server (secure SMTP with TLS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(smtpsAddr)
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
			smtpserver.NewSMTPSService(
				smtpserver.Config{
					Addr:              listen,
					Domain:            smtpsDomain,
					WriteTimeout:      smtpsWriteTimeout,
					ReadTimeout:       smtpsReadTimeout,
					MaxMessageBytes:   smtpsMaxMessageBytes,
					MaxRecipients:     smtpsMaxRecipients,
					AllowInsecureAuth: smtpsAllowInsecureAuth,
				},
				smtpserver.TLSOptions{CertFile: smtpsCert, KeyFile: smtpsKey},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(smtpsCmd)

	smtpsCmd.Flags().StringVar(&smtpsAddr, "listen", ":1465", "listen address")
	smtpsCmd.Flags().StringVar(&smtpsDomain, "domain", "localhost", "SMTP server domain")
	smtpsCmd.Flags().IntVar(&smtpsWriteTimeout, "write-timeout", 10, "write timeout in seconds")
	smtpsCmd.Flags().IntVar(&smtpsReadTimeout, "read-timeout", 10, "read timeout in seconds")
	smtpsCmd.Flags().IntVar(&smtpsMaxMessageBytes, "max-message-bytes", 1024*1024, "max message size in bytes")
	smtpsCmd.Flags().IntVar(&smtpsMaxRecipients, "max-recipients", 50, "maximum number of recipients per message")
	smtpsCmd.Flags().BoolVar(&smtpsAllowInsecureAuth, "allow-insecure-auth", false, "allow authentication without TLS")
	smtpsCmd.Flags().StringVar(&smtpsCert, "cert", "", "path to TLS cert PEM (optional; defaults to ephemeral self-signed)")
	smtpsCmd.Flags().StringVar(&smtpsKey, "key", "", "path to TLS key PEM (optional; defaults to ephemeral self-signed)")
}
