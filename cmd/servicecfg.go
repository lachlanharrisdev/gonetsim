package cmd

import (
	"fmt"
	"path/filepath"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/dnsserver"
	"github.com/lachlanharrisdev/gonetsim/internal/httpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/smtpserver"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
)

func dnsConfig(cfg appconfig.DNSConfig) (dnsserver.Config, error) {
	listen, err := parseAddrPort(cfg.Listen)
	if err != nil {
		return dnsserver.Config{}, fmt.Errorf("dns.listen: %w", err)
	}
	ipv4, err := parseNetipAddr(cfg.IPv4)
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

func smtpConfig(cfg appconfig.SMTPConfig) (smtpserver.Config, error) {
	listen, err := parseAddrPort(cfg.Addr)
	if err != nil {
		return smtpserver.Config{}, fmt.Errorf("smtp.addr: %w", err)
	}
	conf := smtpserver.Config{
		Addr:              listen,
		Domain:            cfg.Domain,
		WriteTimeout:      cfg.WriteTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		MaxMessageBytes:   cfg.MaxMessageBytes,
		MaxRecipients:     cfg.MaxRecipients,
		RequireAuth:       cfg.RequireAuth,
		AllowInsecureAuth: cfg.AllowInsecureAuth,
		LogCredentials:    cfg.LogCredentials,
	}
	if err := conf.Validate(); err != nil {
		return smtpserver.Config{}, fmt.Errorf("smtp: %w", err)
	}
	return conf, nil
}

func smtpsConfig(cfg appconfig.SMTPSConfig, configDir string) (smtpserver.Config, error) {
	listen, err := parseAddrPort(cfg.Addr)
	if err != nil {
		return smtpserver.Config{}, fmt.Errorf("smtps.addr: %w", err)
	}
	certPath := cfg.Cert
	keyPath := cfg.Key
	if certPath == "" && keyPath == "" {
		certPath = filepath.Join(configDir, tlsprovider.PersistedCertFileName)
		keyPath = filepath.Join(configDir, tlsprovider.PersistedKeyFileName)
	}
	conf := smtpserver.Config{
		Addr:              listen,
		Domain:            cfg.Domain,
		WriteTimeout:      cfg.WriteTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		MaxMessageBytes:   cfg.MaxMessageBytes,
		MaxRecipients:     cfg.MaxRecipients,
		RequireAuth:       cfg.RequireAuth,
		AllowInsecureAuth: cfg.AllowInsecureAuth,
		LogCredentials:    cfg.LogCredentials,
		TLS:               &tlsprovider.Config{CertFile: certPath, KeyFile: keyPath},
	}
	if err := conf.Validate(); err != nil {
		return smtpserver.Config{}, fmt.Errorf("smtps: %w", err)
	}
	return conf, nil
}
