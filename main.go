package main

import (
	"os"

	"github.com/lachlanharrisdev/gonetsim/cmd"
)

func main() {
	// set `serve` as root command
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "serve")
	}

	cmd.Execute()
}
