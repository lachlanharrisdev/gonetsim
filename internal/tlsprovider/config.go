package tlsprovider

import (
	"crypto/tls"
	"errors"
)

type Config struct {
	CertFile string
	KeyFile  string

	// controls how the fallback self-signed cert is generated
	// when the cert or key aren't provided
	SelfSigned SelfSignedOptions

	// defaults to tls.VersionTLS12 when zero.
	MinVersion uint16
}

func (c Config) Validate() error {
	if (c.CertFile == "") != (c.KeyFile == "") { // temu xor
		return errors.New("cert and key must be set together")
	}
	return nil
}

func (c Config) TLSConfig() (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	minVersion := c.MinVersion
	if minVersion == 0 {
		minVersion = tls.VersionTLS12
	}

	var cert tls.Certificate
	if c.CertFile != "" {
		loaded, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		cert = loaded
	} else {
		opts := c.SelfSigned
		if len(opts.DNSNames) == 0 && len(opts.IPs) == 0 {
			opts.DNSNames = []string{"localhost"}
		}
		generated, err := GenerateSelfSigned(opts)
		if err != nil {
			return nil, err
		}
		cert = generated
	}

	return &tls.Config{MinVersion: minVersion, Certificates: []tls.Certificate{cert}}, nil
}
