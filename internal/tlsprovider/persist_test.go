package tlsprovider

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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

	cert2, _ := os.ReadFile(cfg.CertFile)
	key2, _ := os.ReadFile(cfg.KeyFile)
	ca2, _ := os.ReadFile(filepath.Join(dir, PersistedCAFileName))

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
