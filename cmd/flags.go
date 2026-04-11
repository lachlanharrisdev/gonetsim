package cmd

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/spf13/cobra"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
)

func parseAddrPort(listen string) (string, error) {
	if listen == "" {
		return "", fmt.Errorf("listen address is required")
	}

	if _, err := net.ResolveTCPAddr("tcp", listen); err != nil {
		return "", fmt.Errorf("invalid listen address %q (expected host:port): %w", listen, err)
	}

	return listen, nil
}

func parseNetipAddr(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid ip %q: %w", s, err)
	}
	return a, nil
}

func parseOptionalNetipAddr(s string) (netip.Addr, error) {
	if s == "" {
		return netip.Addr{}, nil
	}
	return parseNetipAddr(s)
}

type overrideKind int

const (
	overrideString overrideKind = iota
	overrideInt
	overrideBool
)

type flagOverride struct {
	flag string
	key  string
	kind overrideKind
}

func loadConfigWithFlagOverrides(cmd *cobra.Command, defs []flagOverride) (appconfig.LoadResult, error) {
	overrides, err := buildOverrides(cmd, defs)
	if err != nil {
		return appconfig.LoadResult{}, err
	}
	return appconfig.LoadOrCreateWithOverrides(rootConfigPath, overrides)
}

func buildOverrides(cmd *cobra.Command, defs []flagOverride) (map[string]any, error) {
	out := map[string]any{}
	for _, d := range defs {
		if !cmd.Flags().Changed(d.flag) {
			continue
		}
		switch d.kind {
		case overrideString:
			v, err := cmd.Flags().GetString(d.flag)
			if err != nil {
				return nil, fmt.Errorf("read flag %q: %w", d.flag, err)
			}
			out[d.key] = v
		case overrideInt:
			v, err := cmd.Flags().GetInt(d.flag)
			if err != nil {
				return nil, fmt.Errorf("read flag %q: %w", d.flag, err)
			}
			out[d.key] = v
		case overrideBool:
			v, err := cmd.Flags().GetBool(d.flag)
			if err != nil {
				return nil, fmt.Errorf("read flag %q: %w", d.flag, err)
			}
			out[d.key] = v
		default:
			return nil, fmt.Errorf("unknown override kind for flag %q", d.flag)
		}
	}
	return out, nil
}
