package cmd

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/handler"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate configuration and check enabled services can bind their ports",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgRes, err := appconfig.LoadOrCreate(rootConfigPath)
		if err != nil {
			return err
		}
		configDir := filepath.Dir(cfgRes.Path)
		cfg := cfgRes.Config

		out := cmd.OutOrStdout()
		write := func(format string, args ...any) error {
			_, err := fmt.Fprintf(out, format, args...)
			return err
		}

		if err := cfg.Validate(); err != nil {
			_ = write("config: FAIL - %v\n", err)
			return err
		}
		if cfgRes.Created {
			if err := write("config: created %s\n", cfgRes.Path); err != nil {
				return err
			}
		} else {
			if err := write("config: OK (%s)\n", cfgRes.Path); err != nil {
				return err
			}
		}

		checks := []struct {
			name    string
			enabled bool
			run     func() error
			binds   []bindTarget
		}{
			{
				name:    "dns",
				enabled: cfg.DNS.Enabled,
				run: func() error {
					_, err := dnsConfig(cfg.DNS)
					return err
				},
				binds: dnsBindTargets(cfg.DNS.Listen, cfg.DNS.Network),
			},
			{
				name:    "http",
				enabled: cfg.HTTP.Enabled,
				run: func() error {
					_, err := httpConfig(cfg.HTTP)
					return err
				},
				binds: []bindTarget{{net: "tcp", addr: cfg.HTTP.Listen}},
			},
			{
				name:    "https",
				enabled: cfg.HTTPS.Enabled,
				run: func() error {
					_, err := httpsConfig(cfg.HTTPS, configDir)
					return err
				},
				binds: []bindTarget{{net: "tcp", addr: cfg.HTTPS.Listen}},
			},
			{
				name:    "smtp",
				enabled: cfg.SMTP.Enabled,
				run: func() error {
					_, err := smtpConfig(cfg.SMTP)
					return err
				},
				binds: []bindTarget{{net: "tcp", addr: effectiveListen(cfg.SMTP.Listen, cfg.SMTP.Addr)}},
			},
			{
				name:    "smtps",
				enabled: cfg.SMTPS.Enabled,
				run: func() error {
					_, err := smtpsConfig(cfg.SMTPS, configDir)
					return err
				},
				binds: []bindTarget{{net: "tcp", addr: effectiveListen(cfg.SMTPS.Listen, cfg.SMTPS.Addr)}},
			},
		}

		var failures []string
		for _, c := range checks {
			if !c.enabled {
				if err := write("%-8s disabled\n", c.name); err != nil {
					return err
				}
				continue
			}
			if err := c.run(); err != nil {
				failures = append(failures, err.Error())
				if werr := write("%-8s FAIL      %v\n", c.name, err); werr != nil {
					return werr
				}
				continue
			}
			if err := preflightBinds(c.binds); err != nil {
				failures = append(failures, err.Error())
				if werr := write("%-8s FAIL      %v\n", c.name, err); werr != nil {
					return werr
				}
				continue
			}
			if err := write("%-8s OK\n", c.name); err != nil {
				return err
			}
		}

		for _, l := range cfg.Listeners {
			if !l.IsEnabled() {
				if err := write("%-8s disabled\n", l.Name); err != nil {
					return err
				}
				continue
			}

			conf, err := listenerConfig(l, configDir)
			if err != nil {
				failures = append(failures, err.Error())
				if werr := write("%-8s FAIL      %v\n", l.Name, err); werr != nil {
					return werr
				}
				continue
			}

			// compile the lua script to catch errors
			if _, err := handler.New(conf.HandlerSpec, conf.BaseDir); err != nil {
				failures = append(failures, err.Error())
				if werr := write("%-8s FAIL      %v\n", l.Name, err); werr != nil {
					return werr
				}
				continue
			}

			if err := preflightBinds([]bindTarget{{net: conf.Network, addr: conf.Addr}}); err != nil {
				failures = append(failures, err.Error())
				if werr := write("%-8s FAIL      %v\n", l.Name, err); werr != nil {
					return werr
				}
				continue
			}

			if err := write("%-8s OK        %s %s %s\n", l.Name, conf.Network, conf.Addr, conf.HandlerSpec); err != nil {
				return err
			}
		}

		if len(failures) > 0 {
			return fmt.Errorf("check failed:\n  %s", strings.Join(failures, "\n  "))
		}
		return nil
	},
}

type bindTarget struct {
	net  string
	addr string
}

func dnsBindTargets(listen, network string) []bindTarget {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "both":
		return []bindTarget{{net: "udp", addr: listen}, {net: "tcp", addr: listen}}
	case "tcp":
		return []bindTarget{{net: "tcp", addr: listen}}
	default:
		return []bindTarget{{net: "udp", addr: listen}}
	}
}

func preflightBinds(targets []bindTarget) error {
	for _, t := range targets {
		if err := tryBind(t.net, t.addr); err != nil {
			return describeBindError(t, err)
		}
	}
	return nil
}

func tryBind(network, addr string) error {
	switch network {
	case "udp":
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return err
		}
		return pc.Close()
	case "tcp":
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		return ln.Close()
	default:
		return fmt.Errorf("unsupported network %q", network)
	}
}

func describeBindError(t bindTarget, err error) error {
	addr := t.addr
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		if port, ok := parseAddrPortNumber(addr); ok && port < 1024 {
			return fmt.Errorf("cannot bind %s: permission denied (ports below 1024 require elevated privileges on this system)", addr)
		}
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Errorf("cannot bind %s: address already in use", addr)
	}
	return fmt.Errorf("cannot bind %s: %w", addr, err)
}

func parseAddrPortNumber(addr string) (int, bool) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, false
	}
	return port, true
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
