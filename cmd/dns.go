package cmd

import (
	"fmt"
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Run a sinkhole DNS server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSingleServiceCommand(cmd,
			[]flagOverride{
				{flag: "listen", key: "dns.listen", kind: overrideString},
				{flag: "network", key: "dns.network", kind: overrideString},
				{flag: "ipv4", key: "dns.ipv4", kind: overrideString},
				{flag: "ipv6", key: "dns.ipv6", kind: overrideString},
			},
			func(cfg appconfig.Config, logger *slog.Logger) (service.Service, error) {
				listen, err := parseAddrPort(cfg.DNS.Listen)
				if err != nil {
					return nil, fmt.Errorf("dns.listen: %w", err)
				}
				ipv4, err := parseNetipAddr(cfg.DNS.IPv4)
				if err != nil {
					return nil, fmt.Errorf("dns.ipv4: %w", err)
				}
				ipv6, err := parseOptionalNetipAddr(cfg.DNS.IPv6)
				if err != nil {
					return nil, fmt.Errorf("dns.ipv6: %w", err)
				}

				conf := dnsserver.Config{
					Addr:           listen,
					Net:            cfg.DNS.Network,
					SinkholeIPv4:   ipv4,
					SinkholeIPv6:   ipv6,
					SinkholeDomain: cfg.DNS.Domain,
					SinkholeTXT:    cfg.DNS.TXT,
					TTL:            cfg.DNS.TTL,
					Compress:       cfg.DNS.Compress,
				}
				if err := conf.Validate(); err != nil {
					return nil, fmt.Errorf("dns: %w", err)
				}
				return dnsserver.NewService(conf, logger), nil
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	dnsCmd.Flags().String("listen", "", "listen address (overrides config dns.listen)")
	dnsCmd.Flags().String("network", "", "network: udp or tcp (overrides config dns.network)")
	dnsCmd.Flags().String("ipv4", "", "sinkhole IPv4 for A responses (overrides config dns.ipv4)")
	dnsCmd.Flags().String("ipv6", "", "optional sinkhole IPv6 for AAAA responses (overrides config dns.ipv6)")
}
