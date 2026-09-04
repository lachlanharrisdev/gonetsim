package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var rootConfigPath string

var rootCmd = &cobra.Command{
	Use:           "gonetsim [targets...]",
	Short:         "Start all enabled services, or only the given targets",
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runTargets,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal error", "err", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootConfigPath, "config", "", "path to config TOML file (optional)")
	addRunFlags(rootCmd)
}
