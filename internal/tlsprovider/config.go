package tlsprovider

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	PersistedCertFileName = "gonetsim-cert.pem"
	PersistedKeyFileName  = "gonetsim-key.pem"
	PersistedCAFileName   = "gonetsim-ca.pem"
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

	cert, err := c.loadOrGenerateCert()
	if err != nil {
		return nil, err
	}

	return &tls.Config{MinVersion: minVersion, Certificates: []tls.Certificate{cert}}, nil
}

func (c Config) loadOrGenerateCert() (tls.Certificate, error) {
	if c.CertFile == "" {
		opts := defaultSelfSignedOptions(c.SelfSigned)
		return GenerateSelfSigned(opts)
	}

	certExists := fileExists(c.CertFile)
	keyExists := fileExists(c.KeyFile)
	if certExists && keyExists {
		loaded, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return tls.Certificate{}, err
		}
		if isPersistedAutoPair(c.CertFile, c.KeyFile) {
			if err := ensureCAExport(caExportPath(c.CertFile), loaded); err != nil {
				return tls.Certificate{}, err
			}
		}
		return loaded, nil
	}
	if certExists != keyExists {
		return tls.Certificate{}, fmt.Errorf("tls cert and key must exist together: cert=%q key=%q", c.CertFile, c.KeyFile)
	}

	// file missing on disk
	if !isPersistedAutoPair(c.CertFile, c.KeyFile) {
		return tls.Certificate{}, fmt.Errorf("tls cert/key not found: cert=%q key=%q", c.CertFile, c.KeyFile)
	}

	if err := os.MkdirAll(filepath.Dir(c.CertFile), 0o755); err != nil {
		return tls.Certificate{}, err
	}

	opts := defaultSelfSignedOptions(c.SelfSigned)
	certPEM, keyPEM, caPEM, err := GenerateSelfSignedWithCA(opts)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := writeNewFile(c.CertFile, 0o644, certPEM); err != nil {
		return tls.Certificate{}, err
	}
	if err := writeNewFile(c.KeyFile, 0o600, keyPEM); err != nil {
		return tls.Certificate{}, err
	}
	if err := writeNewFile(caExportPath(c.CertFile), 0o644, caPEM); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPEM, keyPEM)
}

func defaultSelfSignedOptions(opts SelfSignedOptions) SelfSignedOptions {
	if len(opts.DNSNames) == 0 && len(opts.IPs) == 0 {
		opts.DNSNames = []string{"localhost"}
	}
	return opts
}

func isPersistedAutoPair(certPath, keyPath string) bool {
	return filepath.Base(certPath) == PersistedCertFileName && filepath.Base(keyPath) == PersistedKeyFileName
}

func caExportPath(certPath string) string {
	return filepath.Join(filepath.Dir(certPath), PersistedCAFileName)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func writeNewFile(path string, perm os.FileMode, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}

	writeErr := func(err error) error {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}

	if _, err := f.Write(data); err != nil {
		return writeErr(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func ensureCAExport(caPath string, cert tls.Certificate) error {
	if fileExists(caPath) {
		return nil
	}
	if len(cert.Certificate) < 2 {
		return nil
	}
	ca, err := x509.ParseCertificate(cert.Certificate[1])
	if err != nil {
		return err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
	return writeNewFile(caPath, 0o644, caPEM)
}
