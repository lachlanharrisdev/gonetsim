package cmd

import (
	"fmt"
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
			},
			func(cfg appconfig.Config, configDir string, logger *slog.Logger) (service.Service, error) {
				listen, err := parseAddrPort(cfg.HTTP.Listen)
				if err != nil {
					return nil, fmt.Errorf("http.listen: %w", err)
				}
				conf := httpserver.Config{Addr: listen, StatusCode: cfg.HTTP.Status}
				if err := conf.Validate(); err != nil {
					return nil, fmt.Errorf("http: %w", err)
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
}
