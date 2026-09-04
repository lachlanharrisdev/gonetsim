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
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	systemConfigPath = "/etc/gonetsim/gonetsim.toml"
	localConfigPath  = "./gonetsim.toml"
)

//go:embed default_config.toml
var defaultConfigTOML []byte

type Config struct {
	General   GeneralConfig    `koanf:"general"`
	DNS       DNSConfig        `koanf:"dns"`
	HTTP      HTTPConfig       `koanf:"http"`
	HTTPS     HTTPSConfig      `koanf:"https"`
	Logging   LoggingConfig    `koanf:"logging"`
	Listeners []ListenerConfig `koanf:"listeners"`
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
	Mode    string `koanf:"mode"`
	RootDir string `koanf:"root_dir"`
}

type HTTPSConfig struct {
	Enabled bool   `koanf:"enabled"`
	Listen  string `koanf:"listen"`
	Status  int    `koanf:"status"`
	Mode    string `koanf:"mode"`
	RootDir string `koanf:"root_dir"`
	Cert    string `koanf:"cert"`
	Key     string `koanf:"key"`
}

type LoggingConfig struct {
	LogFormat string `koanf:"format"`
	Level     string `koanf:"level"`
}

// ListenerConfig is a single [[listeners]] entry; Enabled/Capture default
// to true when unset.
type ListenerConfig struct {
	Name        string        `koanf:"name"`
	Enabled     *bool         `koanf:"enabled"`
	Type        string        `koanf:"type"`
	Listen      string        `koanf:"listen"`
	Handler     string        `koanf:"handler"`
	ReadTimeout time.Duration `koanf:"read_timeout"`
	TLS         bool          `koanf:"tls"`
	TLSCert     string        `koanf:"tls_cert"`
	TLSKey      string        `koanf:"tls_key"`
	Capture     *bool         `koanf:"capture"`
}

func (c ListenerConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

func (c ListenerConfig) ShouldCapture() bool {
	return c.Capture == nil || *c.Capture
}

func Default() Config {
	return Config{
		General: GeneralConfig{ShutdownTimeout: 2 * time.Second},
		DNS: DNSConfig{
			Enabled:  true,
			Listen:   ":53",
			Network:  "udp",
			IPv4:     "auto",
			IPv6:     "::1",
			Domain:   "localhost",
			TXT:      "TXT record response from GoNetSim",
			TTL:      60,
			Compress: false,
		},
		HTTP: HTTPConfig{
			Enabled: true,
			Listen:  ":80",
			Status:  200,
			Mode:    "fake",
		},
		HTTPS: HTTPSConfig{
			Enabled: true,
			Listen:  ":443",
			Status:  200,
			Mode:    "fake",
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

	// deep listener validation happens in the listener package
	names := make(map[string]bool, len(c.Listeners))
	for _, l := range c.Listeners {
		if strings.TrimSpace(l.Name) == "" {
			return errors.New("each listener must have a name")
		}
		if names[l.Name] {
			return fmt.Errorf("listener name %q is used more than once", l.Name)
		}
		names[l.Name] = true
	}

	return nil
}

type LoadResult struct {
	Config  Config
	Path    string
	Created bool
}

func LoadOrCreate(configPath string) (LoadResult, error) {
	return LoadOrCreateWithOverrides(configPath, nil)
}

// LoadOrCreateWithOverrides loads defaults, then the on-disk config file, then applies
// the provided flat overrides (dot-delimited keys).
//
// Validation is intentionally not run here; callers should map the resulting config
// into the isolated service configs (e.g. dnsserver.Config) and call Validate() once
// on those structs before starting services.
func LoadOrCreateWithOverrides(configPath string, overrides map[string]any) (LoadResult, error) {
	resolved, created, err := resolveAndCreate(configPath)
	if err != nil {
		return LoadResult{}, err
	}

	cfg, err := loadConfigFile(resolved, overrides)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Config: cfg, Path: resolved, Created: created}, nil
}

// LoadOptional never creates a config file, unlike LoadOrCreate
func LoadOptional(configPath string, overrides map[string]any) (LoadResult, error) {
	var path string
	if configPath != "" {
		if !fileExists(configPath) {
			return LoadResult{}, fmt.Errorf("config file %q not found", configPath)
		}
		path = configPath
	} else if existing, ok := firstExistingFile(defaultSearchPaths()); ok {
		path = existing
	} else {
		return LoadResult{Config: Default()}, nil
	}

	cfg, err := loadConfigFile(path, overrides)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Config: cfg, Path: path}, nil
}

func loadConfigFile(path string, overrides map[string]any) (Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	if len(overrides) > 0 {
		if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
			return Config{}, fmt.Errorf("load overrides: %w", err)
		}
	}

	out := Default()
	if err := k.UnmarshalWithConf("", &out, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return out, nil
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

	// in order of precedence

	// local config `./gonetsim.toml`
	paths = append(paths, localConfigPath)

	// user config `~/.config/gonetsim/config.toml`
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		paths = append(paths, filepath.Join(d, "gonetsim", "config.toml"))
	}

	// system config `/etc/gonetsim/gonetsim.toml` (unix only)
	if runtime.GOOS != "windows" {
		paths = append(paths, systemConfigPath)
	}

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
