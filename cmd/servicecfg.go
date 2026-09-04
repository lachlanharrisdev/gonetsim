package cmd

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/listener"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
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

const defaultReadTimeout = 30 * time.Second

func listenerConfig(l appconfig.ListenerConfig, configDir string) (listener.Config, error) {
	listen, err := parseAddrPort(l.Listen)
	if err != nil {
		return listener.Config{}, fmt.Errorf("listener %s.listen: %w", l.Name, err)
	}

	readTimeout := l.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = defaultReadTimeout
	}

	network := l.Type
	if network == "" {
		network = "tcp"
	}

	conf := listener.Config{
		Name:        l.Name,
		Network:     network,
		Addr:        listen,
		HandlerSpec: l.Handler,
		ReadTimeout: readTimeout,
		Capture:     l.ShouldCapture(),
		BaseDir:     configDir,
	}

	if l.TLS || l.TLSCert != "" || l.TLSKey != "" {
		certPath, keyPath := l.TLSCert, l.TLSKey
		if certPath == "" && keyPath == "" {
			certPath = filepath.Join(configDir, tlsprovider.PersistedCertFileName)
			keyPath = filepath.Join(configDir, tlsprovider.PersistedKeyFileName)
		}
		conf.TLS = &tlsprovider.Config{CertFile: certPath, KeyFile: keyPath}
	}

	if err := conf.Validate(); err != nil {
		return listener.Config{}, fmt.Errorf("listener %s: %w", l.Name, err)
	}
	return conf, nil
}

func dnsIPv4(s string) (netip.Addr, error) {
	if strings.EqualFold(strings.TrimSpace(s), dnsserver.AutoIPv4) {
		return dnsserver.AutoSinkholeIPv4(), nil
	}
	return parseNetipAddr(s)
}

func dnsConfig(cfg appconfig.DNSConfig) (dnsserver.Config, error) {
	listen, err := parseAddrPort(cfg.Listen)
	if err != nil {
		return dnsserver.Config{}, fmt.Errorf("dns.listen: %w", err)
	}
	ipv4, err := dnsIPv4(cfg.IPv4)
	if err != nil {
		return dnsserver.Config{}, fmt.Errorf("dns.ipv4: %w", err)
	}
	ipv6, err := parseOptionalNetipAddr(cfg.IPv6)
	if err != nil {
		return dnsserver.Config{}, fmt.Errorf("dns.ipv6: %w", err)
	}
	conf := dnsserver.Config{
		Addr:           listen,
		Net:            cfg.Network,
		SinkholeIPv4:   ipv4,
		SinkholeIPv6:   ipv6,
		SinkholeDomain: cfg.Domain,
		SinkholeTXT:    cfg.TXT,
		TTL:            cfg.TTL,
		Compress:       cfg.Compress,
	}
	if err := conf.Validate(); err != nil {
		return dnsserver.Config{}, fmt.Errorf("dns: %w", err)
	}
	return conf, nil
}

func httpConfig(cfg appconfig.HTTPConfig) (httpserver.Config, error) {
	listen, err := parseAddrPort(cfg.Listen)
	if err != nil {
		return httpserver.Config{}, fmt.Errorf("http.listen: %w", err)
	}
	conf := httpserver.Config{
		Addr:       listen,
		StatusCode: cfg.Status,
		Mode:       cfg.Mode,
		RootDir:    cfg.RootDir,
	}
	if err := conf.Validate(); err != nil {
		return httpserver.Config{}, fmt.Errorf("http: %w", err)
	}
	return conf, nil
}

func httpsConfig(cfg appconfig.HTTPSConfig, configDir string) (httpserver.Config, error) {
	listen, err := parseAddrPort(cfg.Listen)
	if err != nil {
		return httpserver.Config{}, fmt.Errorf("https.listen: %w", err)
	}
	certPath := cfg.Cert
	keyPath := cfg.Key
	if certPath == "" && keyPath == "" {
		certPath = filepath.Join(configDir, tlsprovider.PersistedCertFileName)
		keyPath = filepath.Join(configDir, tlsprovider.PersistedKeyFileName)
	}
	conf := httpserver.Config{
		Addr:       listen,
		StatusCode: cfg.Status,
		Mode:       cfg.Mode,
		RootDir:    cfg.RootDir,
		TLS:        &tlsprovider.Config{CertFile: certPath, KeyFile: keyPath},
	}
	if err := conf.Validate(); err != nil {
		return httpserver.Config{}, fmt.Errorf("https: %w", err)
	}
	return conf, nil
}
