package cmd

import (
	"context"
	"log/slog"
	"time"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var (
	httpListen string
	httpStatus int
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Run an HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(httpListen)
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
			httpserver.NewHTTPService(
				httpserver.Config{Addr: listen, StatusCode: httpStatus},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVar(&httpListen, "listen", ":8080", "listen address")
	httpCmd.Flags().IntVar(&httpStatus, "status", 200, "status code to return for all requests")
}
