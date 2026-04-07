package cmd

import (
	"context"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var (
	httpsListen string
	httpsStatus int
	httpsCert   string
	httpsKey    string
)

var httpsCmd = &cobra.Command{
	Use:   "https",
	Short: "Run an HTTPS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(httpsListen)
		if err != nil {
			return err
		}

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()

		manager := service.NewManager(5 * time.Second)
		return manager.RunSingleService(ctx,
			httpserver.NewHTTPSService(
				httpserver.Config{Addr: listen, StatusCode: httpsStatus},
				httpserver.TLSOptions{CertFile: httpsCert, KeyFile: httpsKey},
			),
		)
	},
}

func init() {
	rootCmd.AddCommand(httpsCmd)

	httpsCmd.Flags().StringVar(&httpsListen, "listen", ":8443", "listen address")
	httpsCmd.Flags().IntVar(&httpsStatus, "status", 200, "status code to return for all requests")
	httpsCmd.Flags().StringVar(&httpsCert, "cert", "", "path to TLS cert PEM (optional; defaults to ephemeral self-signed)")
	httpsCmd.Flags().StringVar(&httpsKey, "key", "", "path to TLS key PEM (optional; defaults to ephemeral self-signed)")
}
