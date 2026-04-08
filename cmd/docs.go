package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use: "docs",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("GoNetSim's official documentation can be found at https://gonetsim.lachlanharris.dev")
		fmt.Println("This documentation includes in-depth guides, references for services & detailed specifications\n")
		fmt.Println("The official repository is located at https://github.com/lachlanharrisdev/gonetsim")
		fmt.Println("This is the source for issue tracking, discussions, downloads and contribution guidelines\n")
	},
	Short: "Shows link to GoNetSim documentation",
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
