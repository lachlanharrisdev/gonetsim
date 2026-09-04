package cmd

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func disabledAll(cfg *appconfig.Config) {
	cfg.DNS.Enabled = false
	cfg.HTTP.Enabled = false
	cfg.HTTPS.Enabled = false
	cfg.SMTP.Enabled = false
	cfg.SMTPS.Enabled = false
}

func TestParseTargets(t *testing.T) {
	for _, name := range presetNames {
		spec, err := parseTarget(name)
		if err != nil || spec.kind != targetPreset || spec.preset != name {
			t.Fatalf("parseTarget(%q) = %+v, %v; want preset", name, spec, err)
		}
	}

	spec, err := parseTarget("irc")
	if err != nil || spec.kind != targetListener || spec.name != "irc" {
		t.Fatalf("parseTarget(irc) = %+v, %v; want listener", spec, err)
	}

	cases := []struct {
		arg         string
		handlerSpec string
		name        string
		network     string
		addr        string
	}{
		{"echo@:7777", "builtin:echo", "echo", "tcp", ":7777"},
		{"sink@:9999/udp", "builtin:sink", "sink", "udp", ":9999"},
		{"echo@:1/tcp", "builtin:echo", "echo", "tcp", ":1"},
		{"builtin:echo@:7777", "builtin:echo", "echo", "tcp", ":7777"},
		{"lua:c2.lua@:8080", "lua:c2.lua", "c2", "tcp", ":8080"},
		{"c2.lua@:8080", "lua:c2.lua", "c2", "tcp", ":8080"},
		{"handlers/sub/irc.lua@127.0.0.1:6667", "lua:handlers/sub/irc.lua", "irc", "tcp", "127.0.0.1:6667"},
	}
	for _, tc := range cases {
		spec, err := parseTarget(tc.arg)
		if err != nil || spec.kind != targetInline {
			t.Fatalf("parseTarget(%q) = %+v, %v; want inline", tc.arg, spec, err)
		}
		got := spec.inline
		if got.HandlerSpec != tc.handlerSpec || got.Name != tc.name || got.Network != tc.network || got.Addr != tc.addr {
			t.Fatalf("parseTarget(%q) = %+v; want spec=%s name=%s net=%s addr=%s",
				tc.arg, got, tc.handlerSpec, tc.name, tc.network, tc.addr)
		}
	}

	for _, arg := range []string{
		"@", ":7777@", "echo@",
		"echo@:7777/sctp", "echo@notanaddr", "smtp:foo@:8080", "lua:@:8080", "builtin:@:8080",
	} {
		if _, err := parseTarget(arg); err == nil {
			t.Errorf("parseTarget(%q): expected error", arg)
		}
	}
}

func TestParseSets(t *testing.T) {
	overrides, err := parseSets([]string{
		"http.mode=real",
		"http.status=404",
		"smtp.require_auth=true",
		"dns.enabled=false",
		"general.shutdown_timeout=5s",
	})
	if err != nil {
		t.Fatalf("parseSets: %v", err)
	}
	if overrides["http.mode"] != "real" {
		t.Errorf("http.mode = %v (%T), want string", overrides["http.mode"], overrides["http.mode"])
	}
	if overrides["http.status"] != 404 {
		t.Errorf("http.status = %v (%T), want int", overrides["http.status"], overrides["http.status"])
	}
	if overrides["smtp.require_auth"] != true || overrides["dns.enabled"] != false {
		t.Errorf("bool overrides = %v, %v, want true/false", overrides["smtp.require_auth"], overrides["dns.enabled"])
	}
	if overrides["general.shutdown_timeout"] != "5s" {
		t.Errorf("duration override = %v (%T), want string", overrides["general.shutdown_timeout"], overrides["general.shutdown_timeout"])
	}

	for _, s := range []string{"noequals", "=novalue", ""} {
		if _, err := parseSets([]string{s}); err == nil {
			t.Errorf("parseSets(%q): expected error", s)
		}
	}
}

func TestResolveTargets(t *testing.T) {
	t.Run("all enabled", func(t *testing.T) {
		cfg := appconfig.Default()
		resolved, err := resolveTargets(nil, &cfg, t.TempDir(), t.TempDir(), runOptions{}, testLogger())
		if err != nil {
			t.Fatalf("resolveTargets: %v", err)
		}
		if len(resolved) != len(presetNames) {
			t.Fatalf("expected %d presets, got %d", len(presetNames), len(resolved))
		}
	})

	t.Run("explicit target overrides enabled=false", func(t *testing.T) {
		disabled := false
		cfg := appconfig.Default()
		disabledAll(&cfg)
		cfg.Listeners = []appconfig.ListenerConfig{
			{Name: "irc", Type: "tcp", Listen: "127.0.0.1:0", Handler: "builtin:echo"},
			{Name: "off", Enabled: &disabled, Type: "tcp", Listen: "127.0.0.1:0", Handler: "builtin:sink"},
		}

		resolved, err := resolveTargets(nil, &cfg, t.TempDir(), t.TempDir(), runOptions{}, testLogger())
		if err != nil {
			t.Fatalf("resolveTargets: %v", err)
		}
		if len(resolved) != 1 {
			t.Fatalf("expected disabled listener to be skipped, got %d targets", len(resolved))
		}

		resolved, err = resolveTargets(mustSpecs(t, "off"), &cfg, t.TempDir(), t.TempDir(), runOptions{}, testLogger())
		if err != nil || len(resolved) != 1 {
			t.Fatalf("explicit target: %v, %d targets", err, len(resolved))
		}
	})

	t.Run("unknown target lists alternatives", func(t *testing.T) {
		cfg := appconfig.Default()
		_, err := resolveTargets(mustSpecs(t, "nope"), &cfg, t.TempDir(), t.TempDir(), runOptions{}, testLogger())
		if err == nil || !strings.Contains(err.Error(), "unknown target") ||
			!strings.Contains(err.Error(), "dns") || !strings.Contains(err.Error(), "handler@addr") {
			t.Fatalf("expected helpful unknown-target error, got: %v", err)
		}
	})

	t.Run("inline lua resolves script", func(t *testing.T) {
		cfg := appconfig.Default()
		resolved, err := resolveTargets(mustSpecs(t, "testdata/hello.lua@127.0.0.1:0"), &cfg, t.TempDir(), ".", runOptions{}, testLogger())
		if err != nil || len(resolved) != 1 || resolved[0].display != "hello(127.0.0.1:0)" {
			t.Fatalf("inline lua: %v, %+v", err, resolved)
		}
	})

	t.Run("tls on udp rejected", func(t *testing.T) {
		cfg := appconfig.Default()
		_, err := resolveTargets(mustSpecs(t, "sink@:0/udp"), &cfg, t.TempDir(), t.TempDir(), runOptions{tls: true}, testLogger())
		if err == nil {
			t.Fatalf("expected --tls on udp to be rejected")
		}
	})

	t.Run("listen requires single target", func(t *testing.T) {
		cfg := appconfig.Default()
		opts := runOptions{listen: "127.0.0.1:1234"}
		_, err := resolveTargets(mustSpecs(t, "http", "dns"), &cfg, t.TempDir(), t.TempDir(), opts, testLogger())
		if err == nil {
			t.Fatalf("expected --listen with multiple targets to fail")
		}
	})
}

func TestServiceConfigMapping(t *testing.T) {
	t.Run("preset names match builders", func(t *testing.T) {
		if len(presetTargets) != len(presetNames) {
			t.Fatalf("presetTargets has %d entries, presetNames has %d", len(presetTargets), len(presetNames))
		}
		for i, p := range presetTargets {
			if p.name != presetNames[i] || p.enabled == nil || p.build == nil {
				t.Fatalf("presetTargets[%d] = %q, want %q with enabled/build set", i, p.name, presetNames[i])
			}
		}
	})

	t.Run("effective listen falls back to legacy addr", func(t *testing.T) {
		if got := effectiveListen(":25", ":9999"); got != ":25" {
			t.Errorf("listen wins: got %q", got)
		}
		if got := effectiveListen("", ":9999"); got != ":9999" {
			t.Errorf("addr fallback: got %q", got)
		}
	})

	t.Run("listener type and timeout defaults", func(t *testing.T) {
		l := appconfig.ListenerConfig{Name: "x", Listen: "127.0.0.1:0", Handler: "builtin:echo"}
		conf, err := listenerConfig(l, t.TempDir())
		if err != nil {
			t.Fatalf("listenerConfig: %v", err)
		}
		if conf.Network != "tcp" {
			t.Fatalf("expected type to default to tcp, got %q", conf.Network)
		}
		if conf.ReadTimeout != defaultReadTimeout {
			t.Fatalf("expected default read timeout, got %v", conf.ReadTimeout)
		}
	})

	t.Run("dns auto ipv4", func(t *testing.T) {
		cfg := appconfig.DNSConfig{
			Listen:  "127.0.0.1:0",
			Network: "udp",
			IPv4:    "auto",
			Domain:  "localhost",
			TXT:     "test",
		}
		conf, err := dnsConfig(cfg)
		if err != nil {
			t.Fatalf("dnsConfig(auto): %v", err)
		}
		if !conf.SinkholeIPv4.IsValid() || !conf.SinkholeIPv4.Is4() {
			t.Fatalf("expected auto-detected IPv4, got %v", conf.SinkholeIPv4)
		}

		cfg.IPv4 = "AUTO"
		if _, err := dnsConfig(cfg); err != nil {
			t.Fatalf("dnsConfig(AUTO): %v", err)
		}
		cfg.IPv4 = "203.0.113.10"
		conf, err = dnsConfig(cfg)
		if err != nil {
			t.Fatalf("dnsConfig(explicit): %v", err)
		}
		if conf.SinkholeIPv4.String() != "203.0.113.10" {
			t.Fatalf("expected explicit IP, got %v", conf.SinkholeIPv4)
		}
	})
}

func mustSpecs(t *testing.T, args ...string) []targetSpec {
	t.Helper()
	specs, err := parseTargets(args)
	if err != nil {
		t.Fatalf("parseTargets(%v): %v", args, err)
	}
	return specs
}
