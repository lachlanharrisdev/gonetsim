package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lachlanharrisdev/gonetsim/internal/app"
	"github.com/lachlanharrisdev/gonetsim/internal/logging"
	"github.com/lachlanharrisdev/gonetsim/internal/network"
	"github.com/lachlanharrisdev/gonetsim/internal/pcap"
	"github.com/lachlanharrisdev/gonetsim/internal/script"
	"github.com/lachlanharrisdev/gonetsim/internal/store"
	"github.com/lachlanharrisdev/gonetsim/internal/tlscert"
)

// runTargets is the default command: start one listener per handler@addr.
func runTargets(cmd *cobra.Command, args []string) error {
	specs, err := parseSpecs(args)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return errors.New("no targets given\n\nusage: gonetsim [handler@addr ...] [flags]\n\n  gonetsim echo@:7777\n  gonetsim handlers/irc.lua@:6667\n  gonetsim sink@:9999/udp --pcap ./case.pcapng\n\nsee: gonetsim docs quickstart")
	}
	if runOpts.listen != "" && len(specs) > 1 {
		return fmt.Errorf("--listen requires exactly one target, got %d", len(specs))
	}
	if runOpts.pcap != "" && runOpts.noPcap {
		return errors.New("--pcap and --no-pcap cannot be used together")
	}
	if !runOpts.tls && (runOpts.tlsCert != "" || runOpts.tlsKey != "") {
		return errors.New("--tls-cert/--tls-key require --tls")
	}
	if !runOpts.tls && cmd.Flags().Changed("tls-keylog") {
		return errors.New("--tls-keylog requires --tls")
	}

	logger, err := logging.New(runOpts.logLevel, runOpts.logFormat)
	if err != nil {
		return err
	}

	// Build TLS before opening the capture file so a bad cert/keystore doesn't
	// leave an empty .pcapng behind.
	var tlsConf *tls.Config
	keylogPath := ""
	if runOpts.tls {
		conf := tlscert.Config{CertFile: runOpts.tlsCert, KeyFile: runOpts.tlsKey}
		tlsConf, err = conf.TLSConfig()
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("tls-keylog") {
			if runOpts.tlsKeylog == "" {
				return errors.New("--tls-keylog requires a file path")
			}
			if dir := filepath.Dir(runOpts.tlsKeylog); dir != "." && dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("create output dir %q: %w", dir, err)
				}
			}
			keylogPath = runOpts.tlsKeylog
			klf, err := os.OpenFile(keylogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create tls key log %q: %w", keylogPath, err)
			}
			tlsConf.KeyLogWriter = klf
			logger.Info("key log: " + keylogPath)
			defer func() { _ = klf.Close() }()
		}
		if keylogPath == "" && !runOpts.noPcap {
			logger.Warn("tls listeners are captured as ciphertext on the wire; pass --tls-keylog to save session keys so Wireshark can decrypt the capture")
		}
	}

	var run *pcap.Run
	if !runOpts.noPcap {
		path, err := pcap.DefaultPath(runOpts.pcap)
		if err != nil {
			return err
		}
		manifest, err := json.Marshal(buildManifest(cmd, specs, keylogPath))
		if err != nil {
			// Never let a metadata hiccup block the capture.
			logger.Warn("skip capture manifest: " + err.Error())
			manifest = nil
		}
		run, err = pcap.NewRun(path, string(manifest))
		if err != nil {
			return err
		}
		logger.Info("capture: " + path)
		// Close any orphaned capture if we fail further down; the success path
		// below nils `run` after closing it inline to print stats.
		defer func() {
			if run != nil {
				_ = run.Close()
			}
		}()
	}

	budget := store.NewBudget(store.DefaultTotalLimit)
	global := store.NewStore(budget)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	runners := make([]func(context.Context) error, 0, len(specs))
	displays := make([]string, 0, len(specs))

	for _, sp := range specs {
		if runOpts.tls && sp.network != "tcp" {
			return fmt.Errorf("listener %s: --tls requires a tcp listener", sp.name)
		}
		h, err := script.New(sp.handlerSpec, cwd, budget)
		if err != nil {
			return fmt.Errorf("listener %s handler: %w", sp.name, err)
		}

		addr := sp.addr
		if runOpts.listen != "" {
			addr = runOpts.listen
		}
		idle := defaultIdleTimeout
		if cmd.Flags().Changed("timeout") {
			idle = runOpts.timeout // 0 disables the idle deadline
		}

		log := logging.WithPrefix(logger, sp.name)
		displays = append(displays, displayName(sp, addr))

		if sp.network == "udp" {
			runners = append(runners, func(ctx context.Context) error {
				return network.ServeUDP(ctx, sp.name, addr, run, idle, log, func(ctx context.Context, data []byte, from net.Addr, ses *pcap.Session) ([]byte, error) {
					return h.ServeUDP(ctx, data, from, script.Env{Logger: log, Capture: ses, Global: global})
				})
			})
			continue
		}
		runners = append(runners, func(ctx context.Context) error {
			return network.ServeTCP(ctx, sp.name, addr, tlsConf, run, idle, log, func(ctx context.Context, conn net.Conn, ses *pcap.Session) error {
				return h.ServeTCP(ctx, conn, script.Env{Logger: log, Capture: ses, IdleTimeout: idle, Global: global})
			})
		})
	}

	logger.Info("running: " + strings.Join(displays, " "))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Run app.RunAll in a goroutine so a second Ctrl-C during graceful
	// shutdown can force-exit even when shutdown is taking a long time.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		err = app.RunAll(ctx, runners...)
	}()

	go func() {
		// Wait for either a signal or shutdown to complete.
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
		case <-doneCh:
			return // already finished, nothing to force-exit from
		}
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		select {
		case <-c:
			logger.Warn("forced exit")
			os.Exit(1)
		case <-doneCh:
			return // finished before second signal
		}
	}()

	<-doneCh

	if run != nil {
		packets, first, last := run.Stats()
		path := run.Path()
		_ = run.Close()
		run = nil // tell the cleanup defer to skip
		msg := fmt.Sprintf("capture saved: %s (%d packets, %s)", path, packets, last.Sub(first).Round(time.Millisecond))
		if keylogPath != "" {
			msg += ", keys: " + keylogPath + " (load in Wireshark to decrypt)"
		}
		logger.Info(msg)
	}
	return err
}

func displayName(sp spec, addr string) string {
	display := sp.name + "(" + addr
	if sp.network == "udp" {
		display += "/udp"
	}
	if runOpts.tls {
		display += "+tls"
	}
	return display + ")"
}

// runManifest is embedded in the capture's section header block so the single
// pcapng file is self-describing: which handlers simulated what, and how.
type runManifest struct {
	Version   string             `json:"version"`
	Revision  string             `json:"revision"`
	Date      string             `json:"date"`
	Listen    string             `json:"listen,omitempty"`
	Timeout   string             `json:"timeout"`
	TLS       bool               `json:"tls"`
	TLSCert   string             `json:"tlsCert,omitempty"`
	TLSKey    string             `json:"tlsKey,omitempty"`
	TLSKeylog string             `json:"tlsKeylog,omitempty"`
	LogLevel  string             `json:"logLevel"`
	Listeners []manifestListener `json:"listeners"`
}

type manifestListener struct {
	Handler string `json:"handler"`
	Addr    string `json:"addr"`
	Network string `json:"network"`
}

func buildManifest(cmd *cobra.Command, specs []spec, keylogPath string) runManifest {
	m := runManifest{
		Version:  Version,
		Revision: Revision,
		Date:     Date,
		Listen:   runOpts.listen,
		Timeout:  effectiveTimeout(cmd).String(),
		TLS:      runOpts.tls,
		LogLevel: runOpts.logLevel,
	}
	if runOpts.tlsCert != "" {
		m.TLSCert = filepath.Base(runOpts.tlsCert)
	}
	if runOpts.tlsKey != "" {
		m.TLSKey = filepath.Base(runOpts.tlsKey)
	}
	if keylogPath != "" {
		m.TLSKeylog = filepath.Base(keylogPath)
	}
	for _, sp := range specs {
		m.Listeners = append(m.Listeners, manifestListener{Handler: sp.name, Addr: sp.addr, Network: sp.network})
	}
	return m
}

func effectiveTimeout(cmd *cobra.Command) time.Duration {
	if cmd.Flags().Changed("timeout") {
		return runOpts.timeout
	}
	return defaultIdleTimeout
}
