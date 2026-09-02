package cmd

import (
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/spf13/cobra"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Run an HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSingleServiceCommand(cmd,
			[]flagOverride{
				{flag: "listen", key: "http.listen", kind: overrideString},
				{flag: "status", key: "http.status", kind: overrideInt},
				{flag: "mode", key: "http.mode", kind: overrideString},
				{flag: "root-dir", key: "http.root_dir", kind: overrideString},
			},
			func(cfg appconfig.Config, configDir string, logger *slog.Logger) (service.Service, error) {
				conf, err := httpConfig(cfg.HTTP)
				if err != nil {
					return nil, err
				}
				return httpserver.NewService(conf, logger), nil
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().String("listen", "", "listen address (overrides config http.listen)")
	httpCmd.Flags().Int("status", 0, "status code to return for all requests (overrides config http.status)")
	httpCmd.Flags().String("mode", "", "serve mode: fake or real (overrides config http.mode)")
	httpCmd.Flags().String("root-dir", "", "directory to serve files from in real mode (overrides config http.root_dir)")
}
