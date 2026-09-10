package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Revision = "dev"
	Version  = "dev"
	Date     = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\nRevision: %s\nDate: %s\nOS: %s\nArch: %s\n",
			Version, Revision, Date, runtime.GOOS, runtime.GOARCH)
		return err
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
