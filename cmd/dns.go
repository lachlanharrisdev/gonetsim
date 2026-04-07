package cmd

import (
	"context"
	"time"

	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
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
			Addr:         listen,
			Net:          dnsNetwork,
			SinkholeIPv4: ipv4,
			SinkholeIPv6: ipv6,
		}

		ctx, stop := utils.SignalContext(context.Background())
		defer stop()
		return dnsserver.Run(ctx, conf, dnsserver.RunOptions{ShutdownTimeout: 5 * time.Second})
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	dnsCmd.Flags().StringVar(&dnsListen, "listen", ":5353", "listen address")
	dnsCmd.Flags().StringVar(&dnsNetwork, "network", "udp", "network: udp or tcp")
	dnsCmd.Flags().StringVar(&dnsIPv4, "ipv4", "127.0.0.1", "sinkhole IPv4 for A responses")
	dnsCmd.Flags().StringVar(&dnsIPv6, "ipv6", "::1", "optional sinkhole IPv6 for AAAA responses (empty disables)")
}
