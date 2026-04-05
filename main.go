package main

import (
	"github.com/lachlanharrisdev/gonetsim/cmd"

	"fmt"
)

func main() {
	fmt.Printf("== GoNetSim %s. Copyright (c) 2026 Lachlan Harris ==\n\n", cmd.GetVersionLine())
	cmd.Execute()
}
