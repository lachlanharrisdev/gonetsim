package cmd

import (
	"context"
	"log/slog"
	"time"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

var (
	dnsListen  string
	dnsNetwork string
	dnsIPv4    string
	dnsIPv6    string
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Run a sinkhole DNS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen, err := parseAddrPort(dnsListen)
		if err != nil {
			return err
		}
		ipv4, err := parseNetipAddr(dnsIPv4)
		if err != nil {
			return err
		}
		ipv6, err := parseOptionalNetipAddr(dnsIPv6)
		if err != nil {
			return err
		}

		conf := dnsserver.Config{
			Addr:           listen,
			Net:            dnsNetwork,
			SinkholeIPv4:   ipv4,
			SinkholeIPv6:   ipv6,
			SinkholeDomain: "localhost",
			SinkholeTXT:    "TXT record response from GoNetSim",
			TTL:            60,
			Compress:       false,
		}

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()

		logger, err := observability.NewLogger(appconfig.Default().Logging)
		if err != nil {
			return err
		}
		slog.SetDefault(logger)
		manager := service.NewManager(5*time.Second, logger)
		return manager.RunSingleService(ctx, dnsserver.NewService(conf))
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	dnsCmd.Flags().StringVar(&dnsListen, "listen", ":5353", "listen address")
	dnsCmd.Flags().StringVar(&dnsNetwork, "network", "udp", "network: udp or tcp")
	dnsCmd.Flags().StringVar(&dnsIPv4, "ipv4", "127.0.0.1", "sinkhole IPv4 for A responses")
	dnsCmd.Flags().StringVar(&dnsIPv6, "ipv6", "::1", "optional sinkhole IPv6 for AAAA responses (empty disables)")
}
