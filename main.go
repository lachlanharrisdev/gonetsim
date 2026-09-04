package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/lachlanharrisdev/gonetsim/cmd"
	"github.com/mattn/go-isatty"
)

func main() {
	// the banner goes to stderr so stdout stays clean for command output
	if isatty.IsTerminal(os.Stderr.Fd()) {
		muted := color.New(color.FgHiBlack).SprintFunc()
		fmt.Fprintf(os.Stderr, "%s GoNetSim %s. Copyright (c) 2026 Lachlan Harris %s\n\n", muted("=="), cmd.GetVersionLine(), muted("=="))
	} else {
		fmt.Fprintf(os.Stderr, "== GoNetSim %s. Copyright (c) 2026 Lachlan Harris ==\n\n", cmd.GetVersionLine())
	}
	cmd.Execute()
}
