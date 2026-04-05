package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var licenseCmd = &cobra.Command{
	Use: "license",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("GoNetSim is licensed under the Apache 2.0 License.")
		fmt.Println("")
		fmt.Println("To view this license, or in the event of license changes, please see the LICENSE file at https://github.com/lachlanharrisdev/gonetsim/blob/main/LICENSE")
	},
	Short: "Show license information",
}

func init() {
	rootCmd.AddCommand(licenseCmd)
}
