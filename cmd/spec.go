package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lachlanharrisdev/gonetsim/internal/network"
)

// spec is a parsed inline listener target, e.g. "irc.lua@:6667/udp".
type spec struct {
	name        string
	network     string // "tcp" or "udp"
	addr        string
	handlerSpec string
}

func parseSpecs(args []string) ([]spec, error) {
	out := make([]spec, 0, len(args))
	for _, arg := range args {
		s, err := parseSpec(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseSpec(arg string) (spec, error) {
	i := strings.LastIndex(arg, "@")
	if i <= 0 || i == len(arg)-1 {
		return spec{}, fmt.Errorf("invalid listener %q (expected handler@addr, e.g. echo@:7777)", arg)
	}
	specStr, addr := arg[:i], arg[i+1:]

	netw := "tcp"
	if base, suffix, hasSuffix := strings.Cut(addr, "/"); hasSuffix {
		switch suffix {
		case "tcp", "udp":
			netw = suffix
			addr = base
		default:
			return spec{}, fmt.Errorf("invalid network %q in %q (must be /tcp or /udp)", suffix, arg)
		}
	}
	if _, err := network.ParseAddr(addr); err != nil {
		return spec{}, fmt.Errorf("invalid listen address in %q: %w", arg, err)
	}

	handlerSpec, name, err := resolveHandler(specStr)
	if err != nil {
		return spec{}, fmt.Errorf("%w in %q", err, arg)
	}
	return spec{name: name, network: netw, addr: addr, handlerSpec: handlerSpec}, nil
}

// resolveHandler normalises the part before "@" into a handler spec and a
// display name. Accepts builtin:echo, lua:path, path.lua, or a bare builtin
// name.
func resolveHandler(spec string) (handlerSpec, name string, err error) {
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
