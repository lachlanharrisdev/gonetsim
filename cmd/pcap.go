package cmd

import (
	"github.com/spf13/cobra"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

var pcapCmd = &cobra.Command{
	Use:   "pcap <file.pcap>",
	Short: "Open a PCAP file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := capture.ReadPcap(args[0])
		return err
	},
}

func init() {
	rootCmd.AddCommand(pcapCmd)
}
