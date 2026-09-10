package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	markdown "github.com/lachlanharrisdev/go-std-markdown"

	"github.com/lachlanharrisdev/gonetsim/docs"
)

// docsRaw prints the markdown source untouched instead of rendering it to the
// terminal, for piping into a file or another markdown tool.
var docsRaw bool

var docsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Show built-in documentation (works offline)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		names := docs.Names()
		if len(args) == 0 {
			var b strings.Builder
			b.WriteString("GoNetSim built-in docs. Pick a topic:\n")
			for _, n := range names {
				fmt.Fprintf(&b, "  docs %s\n", n)
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		}
		content, err := docs.Read(args[0])
		if err != nil {
			return err
		}
		if docsRaw {
			_, err := fmt.Fprint(cmd.OutOrStdout(), content)
			return err
		}
		rendered := markdown.Render(content, docsLineWidth(), 0)
		_, err = cmd.OutOrStdout().Write(rendered)
		return err
	},
}

// docsLineWidth picks a rendering width from the terminal. It falls back to 80
// when stdout is not a terminal and caps at 100 so lines stay comfortable on
// wide monitors.
func docsLineWidth() int {
	w, _, err := term.GetSize(uintptr(os.Stdout.Fd()))
	if err != nil || w < 40 {
		return 80
	}
	if w > 100 {
		return 100
	}
	return w
}

func init() {
	docsCmd.Flags().BoolVar(&docsRaw, "raw", false, "print the raw markdown source instead of rendering it")
	rootCmd.AddCommand(docsCmd)
}
