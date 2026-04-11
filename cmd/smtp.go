package cmd

import (
	"fmt"
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/spf13/cobra"
)

var smtpCmd = &cobra.Command{
	Use:   "smtp",
	Short: "Run an SMTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSingleServiceCommand(cmd,
			[]flagOverride{
				{flag: "listen", key: "smtp.addr", kind: overrideString},
				{flag: "domain", key: "smtp.domain", kind: overrideString},
				{flag: "write-timeout", key: "smtp.write_timeout", kind: overrideInt},
				{flag: "read-timeout", key: "smtp.read_timeout", kind: overrideInt},
				{flag: "max-message-bytes", key: "smtp.max_message_bytes", kind: overrideInt},
				{flag: "max-recipients", key: "smtp.max_recipients", kind: overrideInt},
				{flag: "allow-insecure-auth", key: "smtp.allow_insecure_auth", kind: overrideBool},
			},
			func(cfg appconfig.Config, logger *slog.Logger) (service.Service, error) {
				listen, err := parseAddrPort(cfg.SMTP.Addr)
				if err != nil {
					return nil, fmt.Errorf("smtp.addr: %w", err)
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
					return nil, fmt.Errorf("smtp: %w", err)
				}
				return smtpserver.NewService(conf, logger), nil
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(smtpCmd)

	smtpCmd.Flags().String("listen", "", "listen address (overrides config smtp.addr)")
	smtpCmd.Flags().String("domain", "", "SMTP server domain (overrides config smtp.domain)")
	smtpCmd.Flags().Int("write-timeout", 0, "write timeout in seconds (overrides config smtp.write_timeout)")
	smtpCmd.Flags().Int("read-timeout", 0, "read timeout in seconds (overrides config smtp.read_timeout)")
	smtpCmd.Flags().Int("max-message-bytes", 0, "max message size in bytes (overrides config smtp.max_message_bytes)")
	smtpCmd.Flags().Int("max-recipients", 0, "maximum recipients per message (overrides config smtp.max_recipients)")
	smtpCmd.Flags().Bool("allow-insecure-auth", false, "allow auth without TLS (overrides config smtp.allow_insecure_auth)")
}
