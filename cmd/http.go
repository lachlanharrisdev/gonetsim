package cmd

import (
	"context"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
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
		return httpserver.RunHTTP(ctx, httpserver.Config{Addr: listen, StatusCode: httpStatus}, httpserver.RunOptions{ShutdownTimeout: 5 * time.Second})
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)

	httpCmd.Flags().StringVar(&httpListen, "listen", ":8080", "listen address")
	httpCmd.Flags().IntVar(&httpStatus, "status", 200, "status code to return for all requests")
}
