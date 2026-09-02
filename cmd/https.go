package cmd

import (
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
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
				{flag: "mode", key: "https.mode", kind: overrideString},
				{flag: "root-dir", key: "https.root_dir", kind: overrideString},
			},
			func(cfg appconfig.Config, configDir string, logger *slog.Logger) (service.Service, error) {
				conf, err := httpsConfig(cfg.HTTPS, configDir)
				if err != nil {
					return nil, err
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
	httpsCmd.Flags().String("mode", "", "serve mode: fake or real (overrides config https.mode)")
	httpsCmd.Flags().String("root-dir", "", "directory to serve files from in real mode (overrides config https.root_dir)")
}
