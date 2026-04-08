package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const (
	systemConfigPath = "/etc/gonetsim/gonetsim.toml"
	localConfigPath  = "./gonetsim.toml"
)

//go:embed default_config.toml
var defaultConfigTOML []byte

type Config struct {
	General GeneralConfig `koanf:"general"`
	DNS     DNSConfig     `koanf:"dns"`
	HTTP    HTTPConfig    `koanf:"http"`
	HTTPS   HTTPSConfig   `koanf:"https"`
	SMTP    SMTPConfig    `koanf:"smtp"`
	SMTPS   SMTPSConfig   `koanf:"smtps"`
	Logging LoggingConfig `koanf:"logging"`
}

type GeneralConfig struct {
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

type DNSConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Listen   string `koanf:"listen"`
	Network  string `koanf:"network"`
	IPv4     string `koanf:"ipv4"`
	IPv6     string `koanf:"ipv6"`
	Domain   string `koanf:"domain"`
	TXT      string `koanf:"txt"`
	TTL      uint32 `koanf:"ttl"`
	Compress bool   `koanf:"compress"`
}

type HTTPConfig struct {
	Enabled bool   `koanf:"enabled"`
	Listen  string `koanf:"listen"`
	Status  int    `koanf:"status"`
}

type HTTPSConfig struct {
	Enabled bool   `koanf:"enabled"`
	Listen  string `koanf:"listen"`
	Status  int    `koanf:"status"`
	Cert    string `koanf:"cert"`
	Key     string `koanf:"key"`
}

type SMTPConfig struct {
	Enabled           bool   `koanf:"enabled"`
	Addr              string `koanf:"addr"`                // ":1025"
	Domain            string `koanf:"domain"`              // "localhost"
	WriteTimeout      int    `koanf:"write_timeout"`       // 10 seconds
	ReadTimeout       int    `koanf:"read_timeout"`        // 10 seconds
	MaxMessageBytes   int    `koanf:"max_message_bytes"`   // 1024 * 1024
	MaxRecipients     int    `koanf:"max_recipients"`      // 50
	AllowInsecureAuth bool   `koanf:"allow_insecure_auth"` // true
}

type SMTPSConfig struct {
	Enabled           bool   `koanf:"enabled"`
	Addr              string `koanf:"addr"`                // ":1465"
	Domain            string `koanf:"domain"`              // "localhost"
	WriteTimeout      int    `koanf:"write_timeout"`       // 10 seconds
	ReadTimeout       int    `koanf:"read_timeout"`        // 10 seconds
	MaxMessageBytes   int    `koanf:"max_message_bytes"`   // 1024 * 1024
	MaxRecipients     int    `koanf:"max_recipients"`      // 50
	AllowInsecureAuth bool   `koanf:"allow_insecure_auth"` // false (secure)
	Cert              string `koanf:"cert"`                // Optional TLS cert
	Key               string `koanf:"key"`                 // Optional TLS key
}

type LoggingConfig struct {
	LogFormat string `koanf:"format"`
	Level     string `koanf:"level"`
}

func Default() Config {
	return Config{
		General: GeneralConfig{ShutdownTimeout: 2 * time.Second},
		DNS: DNSConfig{
			Enabled:  true,
			Listen:   ":5353",
			Network:  "udp",
			IPv4:     "127.0.0.1",
			IPv6:     "::1",
			Domain:   "localhost",
			TXT:      "TXT record response from GoNetSim",
			TTL:      60,
			Compress: false,
		},
		HTTP: HTTPConfig{
			Enabled: true,
			Listen:  ":8080",
			Status:  200,
		},
		HTTPS: HTTPSConfig{
			Enabled: true,
			Listen:  ":8443",
			Status:  200,
		},
		SMTP: SMTPConfig{
			Enabled:           true,
			Addr:              ":1025",
			Domain:            "localhost",
			WriteTimeout:      10,
			ReadTimeout:       10,
			MaxMessageBytes:   1024 * 1024,
			MaxRecipients:     50,
			AllowInsecureAuth: true,
		},
		SMTPS: SMTPSConfig{
			Enabled:           true,
			Addr:              ":1465",
			Domain:            "localhost",
			WriteTimeout:      10,
			ReadTimeout:       10,
			MaxMessageBytes:   1024 * 1024,
			MaxRecipients:     50,
			AllowInsecureAuth: false,
		},
		Logging: LoggingConfig{
			LogFormat: "text",
			Level:     "info",
		},
	}
}

func (c Config) Validate() error {
	if c.General.ShutdownTimeout <= 0 {
		return errors.New("general.shutdown_timeout must be > 0")
	}

	if c.DNS.Enabled {
		if c.DNS.Listen == "" {
			return errors.New("dns.listen is required when dns.enabled=true")
		}
		if c.DNS.Network == "" {
			return errors.New("dns.network is required when dns.enabled=true")
		}
		if c.DNS.IPv4 == "" {
			return errors.New("dns.ipv4 is required when dns.enabled=true")
		}
	}

	if c.HTTP.Enabled {
		if c.HTTP.Listen == "" {
			return errors.New("http.listen is required when http.enabled=true")
		}
	}

	if c.HTTPS.Enabled {
		if c.HTTPS.Listen == "" {
			return errors.New("https.listen is required when https.enabled=true")
		}
		if (c.HTTPS.Cert == "") != (c.HTTPS.Key == "") {
			return errors.New("https.cert and https.key must be set together")
		}
	}

	if c.SMTP.Enabled {
		if c.SMTP.Addr == "" {
			return errors.New("smtp.addr is required when smtp.enabled=true")
		}
	}

	if c.SMTPS.Enabled {
		if c.SMTPS.Addr == "" {
			return errors.New("smtps.addr is required when smtps.enabled=true")
		}
		if (c.SMTPS.Cert == "") != (c.SMTPS.Key == "") {
			return errors.New("smtps.cert and smtps.key must be set together")
		}
	}

	if !c.DNS.Enabled && !c.HTTP.Enabled && !c.HTTPS.Enabled && !c.SMTP.Enabled && !c.SMTPS.Enabled {
		return errors.New("at least one service must be enabled")
	}

	// logging
	logFormat := strings.ToLower(strings.TrimSpace(c.Logging.LogFormat))
	switch logFormat {
	case "", "text", "json":
		// ok
	default:
		return fmt.Errorf("logging.format must be one of: text, json")
	}
	// default is "info" (see Default()); allow empty for backwards compat
	logLevel := strings.ToLower(strings.TrimSpace(c.Logging.Level))
	switch logLevel {
	case "", "debug", "info", "warn", "warning", "error":
		// ok
	default:
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}

	return nil
}

type LoadResult struct {
	Config  Config
	Path    string
	Created bool
}

func LoadOrCreate(configPath string) (LoadResult, error) {
	resolved, created, err := resolveAndCreate(configPath)
	if err != nil {
		return LoadResult{}, err
	}

	k := koanf.New(".")
	if err := k.Load(structs.Provider(Default(), "koanf"), nil); err != nil {
		return LoadResult{}, fmt.Errorf("load defaults: %w", err)
	}
	if err := k.Load(file.Provider(resolved), toml.Parser()); err != nil {
		return LoadResult{}, fmt.Errorf("load config %q: %w", resolved, err)
	}

	var out Config
	if err := k.UnmarshalWithConf("", &out, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return LoadResult{}, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := out.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("invalid config %q: %w", resolved, err)
	}

	return LoadResult{Config: out, Path: resolved, Created: created}, nil
}

func resolveAndCreate(configPath string) (string, bool, error) {
	if configPath != "" {
		created, err := ensureConfigFile(configPath)
		return configPath, created, err
	}

	if existing, ok := firstExistingFile(defaultSearchPaths()); ok {
		return existing, false, nil
	}

	preferred := preferredDefaultPath()
	created, err := ensureConfigFile(preferred)
	return preferred, created, err
}

func defaultSearchPaths() []string {
	paths := make([]string, 0, 3)

	// in order of precedence from lowest-highest

	// system config `/etc/gonetsim/gonetsim.toml` (unix only)
	if runtime.GOOS != "windows" {
		paths = append(paths, systemConfigPath)
	}

	// user config `~/.config/gonetsim/config.toml` on unix
	// `%APPDATA%\gonetsim\config.toml` on win
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		paths = append(paths, filepath.Join(d, "gonetsim", "config.toml"))
	}

	// local config `./gonetsim.toml`
	paths = append(paths, localConfigPath)
	return paths
}

func preferredDefaultPath() string {
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		return filepath.Join(d, "gonetsim", "config.toml")
	}
	return localConfigPath
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func firstExistingFile(paths []string) (string, bool) {
	for _, p := range paths {
		if fileExists(p) {
			return p, true
		}
	}
	return "", false
}

func ensureConfigFile(path string) (bool, error) {
	if fileExists(path) {
		return false, nil
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, fmt.Errorf("create config dir %q: %w", dir, err)
		}
	}

	// write to a temp file in the same dir to make rename atomic
	tmpFile, err := os.CreateTemp(dir, ".gonetsim-*.toml")
	if err != nil {
		return false, fmt.Errorf("create temp config in %q: %w", dir, err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(defaultConfigTOML); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("write default config %q: %w", tmpName, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("close default config %q: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("chmod default config %q: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("install default config %q: %w", path, err)
	}

	return true, nil
}
