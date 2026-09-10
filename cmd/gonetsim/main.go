package main

import (
	"fmt"
	"os"

	"github.com/lachlanharrisdev/gonetsim/cmd"
)

func main() {
	fmt.Fprintln(os.Stdout, "Copyright (c) 2026 Lachlan Harris. GoNetSim", cmd.Version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gonetsim:", err)
		os.Exit(1)
	}
}
