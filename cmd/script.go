package cmd

import (
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

var scriptCmd = &cobra.Command{
	Use:   "script <file.lua>",
	Short: "Test a Lua handler interactively over stdin/stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		h, err := handler.NewLua(args[0], nil)
		if err != nil {
			return err
		}

		def := appconfig.Default().Logging
		logger, err := observability.NewLogger(observability.Options{Format: def.LogFormat, Level: def.Level})
		if err != nil {
			return err
		}

		global := state.NewStore(nil)
		return h.HandleTCP(cmd.Context(), stdioConn{}, handler.Env{Logger: logger, Global: global})
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
}

type stdioAddr struct{}

func (stdioAddr) Network() string { return "stdio" }
func (stdioAddr) String() string  { return "stdio" }

type stdioConn struct{}

func (stdioConn) Read(p []byte) (int, error)       { return os.Stdin.Read(p) }
func (stdioConn) Write(p []byte) (int, error)      { return os.Stdout.Write(p) }
func (stdioConn) Close() error                     { return nil }
func (stdioConn) LocalAddr() net.Addr              { return stdioAddr{} }
func (stdioConn) RemoteAddr() net.Addr             { return stdioAddr{} }
func (stdioConn) SetDeadline(time.Time) error      { return nil }
func (stdioConn) SetReadDeadline(time.Time) error  { return nil }
func (stdioConn) SetWriteDeadline(time.Time) error { return nil }
