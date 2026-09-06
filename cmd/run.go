package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/state"
)

type runOptions struct {
	sets      []string
	listen    string
	timeout   time.Duration
	tls       bool
	noCapture bool
	output    string
}

var runOpts runOptions

func addRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&runOpts.sets, "set", "s", nil,
		"override a config key (repeatable), e.g. -s http.mode=real -s dns.ipv4=10.0.0.1")
	cmd.Flags().StringVar(&runOpts.listen, "listen", "",
		"override the listen address (requires exactly one target)")
	cmd.Flags().DurationVar(&runOpts.timeout, "timeout", 0,
		"idle read timeout for inline listeners (default 30s)")
	cmd.Flags().BoolVar(&runOpts.tls, "tls", false,
		"wrap inline tcp listeners in TLS with an in-memory self-signed certificate")
	cmd.Flags().BoolVar(&runOpts.noCapture, "no-capture", false,
		"don't write a capture file for this run")
	cmd.Flags().StringVar(&runOpts.output, "output", "",
		"write the run capture to this pcapng file instead of the default runs directory")
}

var runCmd = &cobra.Command{
	Use:           "run [targets...]",
	Short:         "Run the given targets; presets, listener names, or inline handler@addr",
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runTargets,
}

func init() {
	rootCmd.AddCommand(runCmd)
	addRunFlags(runCmd)
}

func runTargets(cmd *cobra.Command, args []string) error {
	specs, err := parseTargets(args)
	if err != nil {
		return err
	}

	overrides, err := parseSets(runOpts.sets)
	if err != nil {
		return err
	}

	var cfgRes appconfig.LoadResult
	if len(specs) == 0 {
		cfgRes, err = appconfig.LoadOrCreateWithOverrides(rootConfigPath, overrides)
	} else {
		cfgRes, err = appconfig.LoadOptional(rootConfigPath, overrides)
	}
	if err != nil {
		return err
	}
	configDir := filepath.Dir(cfgRes.Path)
	cfg := cfgRes.Config
	if err := cfg.Validate(); err != nil {
		return err
	}

	logger, err := observability.NewLogger(observability.Options{Format: cfg.Logging.LogFormat, Level: cfg.Logging.Level})
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	if cfgRes.Created {
		logger.Info("config created", "path", cfgRes.Path)
	}
	if cfgRes.Path != "" {
		logger.Info("config loaded", "path", cfgRes.Path)
	} else {
		logger.Info("no config file found, using defaults")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	limit, err := appconfig.ParseSize(cfg.State.TotalLimit)
	if err != nil {
		return err
	}
	global := state.NewStore(state.NewBudget(limit))

	var run *capture.Run
	if !runOpts.noCapture {
		path, err := capture.RunPath(runOpts.output)
		if err != nil {
			return err
		}
		run, err = capture.NewRun(path)
		if err != nil {
			return err
		}
		logger.Info("capture", "path", path)
	}

	resolved, err := resolveTargets(specs, &cfg, configDir, cwd, runOpts, logger, global, run)
	if err != nil {
		if run != nil {
			_ = run.Close()
		}
		return err
	}
	if len(resolved) == 0 {
		if run != nil {
			_ = run.Close()
		}
		return fmt.Errorf("at least one service must be enabled")
	}

	displays := make([]string, len(resolved))
	manager := service.NewManager(cfg.General.ShutdownTimeout, logger)
	for i, rt := range resolved {
		displays[i] = rt.display
		manager.Add(rt.svc)
	}
	logger.Info("running", "targets", strings.Join(displays, " "))

	runErr := manager.RunAll(runCtx)
	if run != nil {
		packets, first, last := run.Stats()
		path := run.Path()
		_ = run.Close()
		logger.Info("capture saved", "path", path, "packets", packets, "duration", last.Sub(first).Round(time.Millisecond))
	}
	return runErr
}
