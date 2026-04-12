package tlsprovider

import (
	"bytes"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// / <summary>
// / basic test for SSL/TLS cert gen. generates a cert & performs basic checks
// / </summary>
func TestGenerateSelfSigned_SaneCertificate(t *testing.T) {
	cert, err := GenerateSelfSigned(SelfSignedOptions{
		DNSNames: []string{"localhost", "example.test"},
		IPs:      []net.IP{net.ParseIP("127.0.0.1")},
		ValidFor: 2 * time.Hour,
	})
	if err != nil {
		// failed with error
		t.Fatalf("GenerateSelfSigned: %v", err)
	}
	if len(cert.Certificate) == 0 {
		// failed to generate certificate
		t.Fatalf("expected at least one certificate")
	}
	if cert.PrivateKey == nil {
		// failed to generate private key
		t.Fatalf("expected PrivateKey to be set")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		// failed to parse generated certificate with error
		t.Fatalf("ParseCertificate: %v", err)
	}

	if time.Until(leaf.NotAfter) <= 0 {
		// failed to generate a certificate that is currently valid
		t.Fatalf("expected certificate to be currently valid")
	}

	if leaf.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment) == 0 {
		// failed to generate a certificate with appropriate key usage for TLS server
		t.Fatalf("expected KeyUsage to include digital signature and/or key encipherment, got %v", leaf.KeyUsage)
	}
	if len(leaf.ExtKeyUsage) == 0 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		// failed to generate a certificate with appropriate extended key usage for TLS server
		t.Fatalf("expected ExtKeyUsage to include server auth, got %v", leaf.ExtKeyUsage)
	}

	if len(cert.Certificate) < 2 {
		t.Fatalf("expected a CA certificate in the chain")
	}
	ca, err := x509.ParseCertificate(cert.Certificate[1])
	if err != nil {
		t.Fatalf("ParseCertificate (ca): %v", err)
	}
	if !ca.IsCA {
		t.Fatalf("expected CA certificate")
	}

	// excluded potential checks:
	// - leaf.Subject.CommonName != "gonetsim"
	// - !leaf.NotAfter.After(leaf.NotBefore)

}

func TestTLSConfig_AutoPersistedPair_Reused(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		CertFile: filepath.Join(dir, PersistedCertFileName),
		KeyFile:  filepath.Join(dir, PersistedKeyFileName),
	}

	_, err := cfg.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig (first): %v", err)
	}

	cert1, err := os.ReadFile(cfg.CertFile)
	if err != nil {
		t.Fatalf("ReadFile(cert): %v", err)
	}
	key1, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		t.Fatalf("ReadFile(key): %v", err)
	}
	ca1, err := os.ReadFile(filepath.Join(dir, PersistedCAFileName))
	if err != nil {
		t.Fatalf("ReadFile(ca): %v", err)
	}

	_, err = cfg.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig (second): %v", err)
	}

	cert2, err := os.ReadFile(cfg.CertFile)
	if err != nil {
		t.Fatalf("ReadFile(cert, second): %v", err)
	}
	key2, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		t.Fatalf("ReadFile(key, second): %v", err)
	}
	ca2, err := os.ReadFile(filepath.Join(dir, PersistedCAFileName))
	if err != nil {
		t.Fatalf("ReadFile(ca, second): %v", err)
	}

	if !bytes.Equal(cert1, cert2) {
		t.Fatalf("expected cert to be reused")
	}
	if !bytes.Equal(key1, key2) {
		t.Fatalf("expected key to be reused")
	}
	if !bytes.Equal(ca1, ca2) {
		t.Fatalf("expected CA to be reused")
	}
}
