package cmd

import (
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/spf13/cobra"
)

var smtpsCmd = &cobra.Command{
	Use:   "smtps",
	Short: "Run an SMTPS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSingleServiceCommand(cmd,
			[]flagOverride{
				{flag: "listen", key: "smtps.addr", kind: overrideString},
				{flag: "domain", key: "smtps.domain", kind: overrideString},
				{flag: "write-timeout", key: "smtps.write_timeout", kind: overrideInt},
				{flag: "read-timeout", key: "smtps.read_timeout", kind: overrideInt},
				{flag: "max-message-bytes", key: "smtps.max_message_bytes", kind: overrideInt},
				{flag: "max-recipients", key: "smtps.max_recipients", kind: overrideInt},
				{flag: "allow-insecure-auth", key: "smtps.allow_insecure_auth", kind: overrideBool},
				{flag: "log-credentials", key: "smtps.log_credentials", kind: overrideBool},
				{flag: "cert", key: "smtps.cert", kind: overrideString},
				{flag: "key", key: "smtps.key", kind: overrideString},
			},
			func(cfg appconfig.Config, configDir string, logger *slog.Logger) (service.Service, error) {
				conf, err := smtpsConfig(cfg.SMTPS, configDir)
				if err != nil {
					return nil, err
				}
				return smtpserver.NewService(conf, logger), nil
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(smtpsCmd)

	smtpsCmd.Flags().String("listen", "", "listen address (overrides config smtps.addr)")
	smtpsCmd.Flags().String("domain", "", "SMTP server domain (overrides config smtps.domain)")
	smtpsCmd.Flags().Int("write-timeout", 0, "write timeout in seconds (overrides config smtps.write_timeout)")
	smtpsCmd.Flags().Int("read-timeout", 0, "read timeout in seconds (overrides config smtps.read_timeout)")
	smtpsCmd.Flags().Int("max-message-bytes", 0, "max message size in bytes (overrides config smtps.max_message_bytes)")
	smtpsCmd.Flags().Int("max-recipients", 0, "maximum recipients per message (overrides config smtps.max_recipients)")
	smtpsCmd.Flags().Bool("allow-insecure-auth", false, "allow auth without TLS (overrides config smtps.allow_insecure_auth)")
	smtpsCmd.Flags().Bool("log-credentials", false, "log plaintext passwords on auth (overrides config smtps.log_credentials)")
	smtpsCmd.Flags().String("cert", "", "path to TLS cert PEM (overrides config smtps.cert)")
	smtpsCmd.Flags().String("key", "", "path to TLS key PEM (overrides config smtps.key)")
}
