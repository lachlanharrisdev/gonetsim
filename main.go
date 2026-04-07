package main

import (
	"github.com/lachlanharrisdev/gonetsim/cmd"

	"github.com/mattn/go-isatty"

	"fmt"
	"os"

	"github.com/fatih/color"
)

func main() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		muted := color.New(color.FgHiBlack).SprintFunc()
		fmt.Printf("%s GoNetSim %s. Copyright (c) 2026 Lachlan Harris %s\n\n", muted("=="), cmd.GetVersionLine(), muted("=="))
	} else {
		fmt.Printf("== GoNetSim %s. Copyright (c) 2026 Lachlan Harris ==\n\n", cmd.GetVersionLine())
	}
	cmd.Execute()
}
