package main

import (
	"github.com/lachlanharrisdev/gonetsim/cmd"

	"fmt"
)

func main() {
	fmt.Println(fmt.Sprintf("== GoNetSim %s. Copyright (c) 2026 Lachlan Harris ==", cmd.GetVersionLine()))
	fmt.Println("")
	cmd.Execute()
}
