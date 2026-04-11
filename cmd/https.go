package cmd

import (
	"fmt"
	"log/slog"
	"path/filepath"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
	"github.com/spf13/cobra"
)

var httpsCmd = &cobra.Command{
	Use:   "https",
	Short: "Run an HTTPS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSingleServiceCommand(cmd,
			[]flagOverride{
				{flag: "listen", key: "https.listen", kind: overrideString},
				{flag: "status", key: "https.status", kind: overrideInt},
				{flag: "cert", key: "https.cert", kind: overrideString},
				{flag: "key", key: "https.key", kind: overrideString},
			},
			func(cfg appconfig.Config, configDir string, logger *slog.Logger) (service.Service, error) {
				listen, err := parseAddrPort(cfg.HTTPS.Listen)
				if err != nil {
					return nil, fmt.Errorf("https.listen: %w", err)
				}

				certPath := cfg.HTTPS.Cert
				keyPath := cfg.HTTPS.Key
				if certPath == "" && keyPath == "" {
					certPath = filepath.Join(configDir, tlsprovider.PersistedCertFileName)
					keyPath = filepath.Join(configDir, tlsprovider.PersistedKeyFileName)
				}

				conf := httpserver.Config{
					Addr:       listen,
					StatusCode: cfg.HTTPS.Status,
					TLS:        &tlsprovider.Config{CertFile: certPath, KeyFile: keyPath},
				}
				if err := conf.Validate(); err != nil {
					return nil, fmt.Errorf("https: %w", err)
				}
				return httpserver.NewService(conf, logger), nil
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(httpsCmd)

	httpsCmd.Flags().String("listen", "", "listen address (overrides config https.listen)")
	httpsCmd.Flags().Int("status", 0, "status code to return for all requests (overrides config https.status)")
	httpsCmd.Flags().String("cert", "", "path to TLS cert PEM (overrides config https.cert)")
	httpsCmd.Flags().String("key", "", "path to TLS key PEM (overrides config https.key)")
}
