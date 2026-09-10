// Package cmd wires the gonetsim command line: the default run command plus
// the tls, version and docs subcommands.
package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const defaultIdleTimeout = 30 * time.Second

type runOptions struct {
	listen    string
	timeout   time.Duration
	tls       bool
	tlsCert   string
	tlsKey    string
	tlsKeylog string
	pcap      string
	noPcap    bool
	logLevel  string
	logFormat string
}

var runOpts runOptions

// timeoutValue accepts either a bare number of seconds or a Go duration, so
// --timeout 5 and --timeout 5s both mean five seconds.
type timeoutValue struct{ d *time.Duration }

func newTimeoutValue(d *time.Duration) *timeoutValue { return &timeoutValue{d: d} }

func (v *timeoutValue) Set(s string) error {
	if d, err := time.ParseDuration(s); err == nil {
		*v.d = d
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid duration %q (use e.g. 5, 5s, 1m30s)", s)
	}
	*v.d = time.Duration(n * float64(time.Second))
	return nil
}

func (v *timeoutValue) String() string { return v.d.String() }

func (v *timeoutValue) Type() string { return "duration" }

var rootCmd = &cobra.Command{
	Use:           "gonetsim [handler@addr ...]",
	Short:         "A lightweight, sandboxed Lua network simulator",
	Long:          "Run sandboxed Lua handlers (or the echo/sink builtins) on\nTCP or UDP listeners, with terminal logs and pcapng capture.\n\n  gonetsim irc.lua@:6667\n  gonetsim echo@:7777 sink@:9999/udp --pcap ./case.pcapng",
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
	RunE:          runTargets,
}

// Execute runs the root command, returning any error instead of exiting.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	flags := rootCmd.Flags()
	flags.StringVar(&runOpts.logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	flags.StringVar(&runOpts.logFormat, "log-format", "text", "log format (text, json)")
	flags.StringVar(&runOpts.listen, "listen", "", "override the listen address (requires exactly one target)")
	flags.Var(newTimeoutValue(&runOpts.timeout), "timeout", "connection idle timeout (default 30s; 0 disables)")
	flags.BoolVar(&runOpts.tls, "tls", false, "wrap tcp listeners in TLS (self-signed unless --tls-cert/--tls-key are given)")
	flags.StringVar(&runOpts.tlsCert, "tls-cert", "", "TLS certificate file, used with --tls")
	flags.StringVar(&runOpts.tlsKey, "tls-key", "", "TLS key file, used with --tls")
	flags.StringVar(&runOpts.tlsKeylog, "tls-keylog", "", "write TLS session keys to f so Wireshark can decrypt the capture (SSLKEYLOGFILE format); requires --tls")
	flags.StringVar(&runOpts.pcap, "pcap", "", "write the run capture to this pcapng file (default ./<id>.pcapng)")
	flags.BoolVar(&runOpts.noPcap, "no-pcap", false, "don't write a capture file")
}
