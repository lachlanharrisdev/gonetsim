package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/listener"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

var presetNames = []string{"dns", "http", "https", "smtp", "smtps"}

type targetKind int

const (
	targetPreset targetKind = iota
	targetListener
	targetInline
)

type targetSpec struct {
	raw    string
	kind   targetKind
	preset string          // targetPreset
	name   string          // targetListener
	inline listener.Config // targetInline
}

func parseTargets(args []string) ([]targetSpec, error) {
	out := make([]targetSpec, 0, len(args))
	for _, arg := range args {
		spec, err := parseTarget(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}

func parseTarget(arg string) (targetSpec, error) {
	if strings.Contains(arg, "@") {
		return parseInlineTarget(arg)
	}
	for _, p := range presetNames {
		if arg == p {
			return targetSpec{raw: arg, kind: targetPreset, preset: p}, nil
		}
	}
	return targetSpec{raw: arg, kind: targetListener, name: arg}, nil
}

func parseInlineTarget(arg string) (targetSpec, error) {
	i := strings.LastIndex(arg, "@")
	if i <= 0 || i == len(arg)-1 {
		return targetSpec{}, fmt.Errorf("invalid listener %q (expected handler@addr, e.g. echo@:7777)", arg)
	}
	spec, addr := arg[:i], arg[i+1:]

	network := "tcp"
	if base, suffix, hasSuffix := strings.Cut(addr, "/"); hasSuffix {
		switch suffix {
		case "tcp", "udp":
			network = suffix
			addr = base
		default:
			return targetSpec{}, fmt.Errorf("invalid network %q in %q (must be /tcp or /udp)", suffix, arg)
		}
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return targetSpec{}, fmt.Errorf("invalid listen address %q in %q (expected host:port): %w", addr, arg, err)
	}

	handlerSpec, name, err := resolveInlineHandler(spec)
	if err != nil {
		return targetSpec{}, fmt.Errorf("%w in %q", err, arg)
	}

	return targetSpec{
		raw:  arg,
		kind: targetInline,
		inline: listener.Config{
			Name:        name,
			Network:     network,
			Addr:        addr,
			HandlerSpec: handlerSpec,
			Capture:     true,
		},
	}, nil
}

func resolveInlineHandler(spec string) (string, string, error) {
	if scheme, value, hasScheme := strings.Cut(spec, ":"); hasScheme {
		switch scheme {
		case "builtin":
			if value == "" {
				return "", "", fmt.Errorf("empty builtin handler %q", spec)
			}
			return spec, value, nil
		case "lua":
			if value == "" {
				return "", "", fmt.Errorf("empty lua script %q", spec)
			}
			return spec, luaName(value), nil
		default:
			return "", "", fmt.Errorf("unknown handler scheme %q (must be builtin or lua)", scheme)
		}
	}
	if strings.HasSuffix(spec, ".lua") {
		return "lua:" + spec, luaName(spec), nil
	}
	return "builtin:" + spec, spec, nil
}

func luaName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".lua")
}

func parseSets(sets []string) (map[string]any, error) {
	out := make(map[string]any, len(sets))
	for _, s := range sets {
		key, value, ok := strings.Cut(s, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid --set %q (expected key=value)", s)
		}
		switch {
		case value == "true":
			out[key] = true
		case value == "false":
			out[key] = false
		default:
			if n, err := strconv.Atoi(value); err == nil {
				out[key] = n
			} else {
				out[key] = value
			}
		}
	}
	return out, nil
}

type resolvedTarget struct {
	svc     service.Service
	display string
}

func resolveTargets(specs []targetSpec, cfg *appconfig.Config, configDir, cwd string, opts runOptions, logger *slog.Logger) ([]resolvedTarget, error) {
	if len(specs) == 0 {
		return resolveAll(cfg, configDir, opts, logger)
	}

	if opts.listen != "" && len(specs) > 1 {
		return nil, fmt.Errorf("--listen requires exactly one target, got %d", len(specs))
	}

	out := make([]resolvedTarget, 0, len(specs))
	for _, spec := range specs {
		rt, err := resolveOne(spec, cfg, configDir, cwd, opts, logger)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, nil
}

func resolveAll(cfg *appconfig.Config, configDir string, opts runOptions, logger *slog.Logger) ([]resolvedTarget, error) {
	out := make([]resolvedTarget, 0, len(presetTargets)+len(cfg.Listeners))
	for _, p := range presetTargets {
		if !p.enabled(cfg) {
			continue
		}
		svc, display, err := p.build(cfg, configDir, opts, logger)
		if err != nil {
			return nil, err
		}
		out = append(out, resolvedTarget{svc: svc, display: display})
	}

	for _, l := range cfg.Listeners {
		if !l.IsEnabled() {
			continue
		}
		rt, err := resolveOne(targetSpec{raw: l.Name, kind: targetListener, name: l.Name}, cfg, configDir, "", opts, logger)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, nil
}

func resolveOne(spec targetSpec, cfg *appconfig.Config, configDir, cwd string, opts runOptions, logger *slog.Logger) (resolvedTarget, error) {
	switch spec.kind {
	case targetPreset:
		for _, p := range presetTargets {
			if p.name != spec.preset {
				continue
			}
			svc, display, err := p.build(cfg, configDir, opts, logger)
			if err != nil {
				return resolvedTarget{}, err
			}
			return resolvedTarget{svc: svc, display: display}, nil
		}
		return resolvedTarget{}, fmt.Errorf("unknown target %q (available: %s)", spec.raw, availableTargets(cfg))

	case targetListener:
		var entry *appconfig.ListenerConfig
		for i := range cfg.Listeners {
			if cfg.Listeners[i].Name == spec.name {
				entry = &cfg.Listeners[i]
				break
			}
		}
		if entry == nil {
			return resolvedTarget{}, fmt.Errorf("unknown target %q (available: %s)", spec.raw, availableTargets(cfg))
		}
		conf, err := listenerConfig(*entry, configDir)
		if err != nil {
			return resolvedTarget{}, err
		}
		if err := applyListenerRunOptions(&conf, opts, opts.listen); err != nil {
			return resolvedTarget{}, err
		}
		return buildListener(conf, logger)

	case targetInline:
		conf := spec.inline
		conf.BaseDir = cwd
		if err := applyListenerRunOptions(&conf, opts, opts.listen); err != nil {
			return resolvedTarget{}, err
		}
		return buildListener(conf, logger)

	default:
		return resolvedTarget{}, fmt.Errorf("unknown target kind %d", spec.kind)
	}
}

func applyListenerRunOptions(conf *listener.Config, opts runOptions, listen string) error {
	if listen != "" {
		conf.Addr = listen
	}
	if conf.ReadTimeout <= 0 {
		conf.ReadTimeout = defaultReadTimeout
	}
	if opts.timeout > 0 {
		conf.ReadTimeout = opts.timeout
	}
	if opts.noCapture {
		conf.Capture = false
	}
	if opts.artifacts != "" {
		conf.CaptureDir = opts.artifacts
	}
	if opts.tls {
		if conf.Network != "tcp" {
			return fmt.Errorf("listener %s: --tls requires a tcp listener", conf.Name)
		}
		// empty config requires an ephemeral self signed cert
		conf.TLS = &tlsprovider.Config{}
	}
	return nil
}

func buildListener(conf listener.Config, logger *slog.Logger) (resolvedTarget, error) {
	svc, err := listener.NewService(conf, logger)
	if err != nil {
		return resolvedTarget{}, err
	}
	return resolvedTarget{svc: svc, display: listenerDisplay(conf)}, nil
}

func listenerDisplay(conf listener.Config) string {
	display := conf.Name + "(" + conf.Addr
	if conf.Network == "udp" {
		display += "/udp"
	}
	if conf.TLS != nil {
		display += "+tls"
	}
	return display + ")"
}

var presetTargets = []struct {
	name    string
	enabled func(c *appconfig.Config) bool
	build   func(c *appconfig.Config, configDir string, opts runOptions, logger *slog.Logger) (service.Service, string, error)
}{
	{
		name:    "dns",
		enabled: func(c *appconfig.Config) bool { return c.DNS.Enabled },
		build: func(c *appconfig.Config, _ string, opts runOptions, logger *slog.Logger) (service.Service, string, error) {
			if opts.listen != "" {
				c.DNS.Listen = opts.listen
			}
			conf, err := dnsConfig(c.DNS)
			if err != nil {
				return nil, "", err
			}
			return dnsserver.NewService(conf, logger), fmt.Sprintf("dns(%s/%s)", conf.Addr, netLabel(conf.Net)), nil
		},
	},
	{
		name:    "http",
		enabled: func(c *appconfig.Config) bool { return c.HTTP.Enabled },
		build: func(c *appconfig.Config, _ string, opts runOptions, logger *slog.Logger) (service.Service, string, error) {
			if opts.listen != "" {
				c.HTTP.Listen = opts.listen
			}
			conf, err := httpConfig(c.HTTP)
			if err != nil {
				return nil, "", err
			}
			return httpserver.NewService(conf, logger), fmt.Sprintf("http(%s)", conf.Addr), nil
		},
	},
	{
		name:    "https",
		enabled: func(c *appconfig.Config) bool { return c.HTTPS.Enabled },
		build: func(c *appconfig.Config, configDir string, opts runOptions, logger *slog.Logger) (service.Service, string, error) {
			if opts.listen != "" {
				c.HTTPS.Listen = opts.listen
			}
			conf, err := httpsConfig(c.HTTPS, configDir)
			if err != nil {
				return nil, "", err
			}
			return httpserver.NewService(conf, logger), fmt.Sprintf("https(%s)", conf.Addr), nil
		},
	},
	{
		name:    "smtp",
		enabled: func(c *appconfig.Config) bool { return c.SMTP.Enabled },
		build: func(c *appconfig.Config, _ string, opts runOptions, logger *slog.Logger) (service.Service, string, error) {
			if opts.listen != "" {
				c.SMTP.Listen = opts.listen
			}
			conf, err := smtpConfig(c.SMTP)
			if err != nil {
				return nil, "", err
			}
			return smtpserver.NewService(conf, logger), fmt.Sprintf("smtp(%s)", conf.Addr), nil
		},
	},
	{
		name:    "smtps",
		enabled: func(c *appconfig.Config) bool { return c.SMTPS.Enabled },
		build: func(c *appconfig.Config, configDir string, opts runOptions, logger *slog.Logger) (service.Service, string, error) {
			if opts.listen != "" {
				c.SMTPS.Listen = opts.listen
			}
			conf, err := smtpsConfig(c.SMTPS, configDir)
			if err != nil {
				return nil, "", err
			}
			return smtpserver.NewService(conf, logger), fmt.Sprintf("smtps(%s)", conf.Addr), nil
		},
	},
}

func netLabel(net string) string {
	switch strings.ToLower(strings.TrimSpace(net)) {
	case "both":
		return "udp+tcp"
	case "tcp":
		return "tcp"
	default:
		return "udp"
	}
}

func availableTargets(cfg *appconfig.Config) string {
	names := append([]string{}, presetNames...)
	for _, l := range cfg.Listeners {
		names = append(names, l.Name)
	}
	return fmt.Sprintf("%s; or handler@addr for an inline listener, e.g. echo@:7777", strings.Join(names, ", "))
}
