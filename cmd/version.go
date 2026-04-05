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

func GetVersion() string {
	return fmt.Sprintf(`Version: %s
Revision: %s
Date: %s
OS: %s
Arch: %s`, Version, Revision, Date, runtime.GOOS, runtime.GOARCH)
}

// alternative that returns a single line string
func GetVersionLine() string {
	return fmt.Sprintf("%s (%s %s)", Version, runtime.GOOS, runtime.GOARCH)
}

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetVersion())
	},
	Short: "Show version info",
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
