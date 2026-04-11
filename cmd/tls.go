package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/tlsprovider"
	"github.com/spf13/cobra"
)

var tlsVerifyOnly bool

var tlsCmd = &cobra.Command{
	Use:   "tls",
	Short: "Generate and verify persisted TLS certificates",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgRes, err := appconfig.LoadOrCreate(rootConfigPath)
		if err != nil {
			return err
		}
		configDir := filepath.Dir(cfgRes.Path)

		certPath := filepath.Join(configDir, tlsprovider.PersistedCertFileName)
		keyPath := filepath.Join(configDir, tlsprovider.PersistedKeyFileName)
		caPath := filepath.Join(configDir, tlsprovider.PersistedCAFileName)

		if !tlsVerifyOnly {
			// Generates the persisted auto pair if missing.
			_, err := (&tlsprovider.Config{CertFile: certPath, KeyFile: keyPath}).TLSConfig()
			if err != nil {
				return err
			}
		}

		if err := verifyKeyPair(certPath, keyPath, caPath); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "TLS OK")
		fmt.Fprintln(cmd.OutOrStdout(), "cert:", certPath)
		fmt.Fprintln(cmd.OutOrStdout(), "key: ", keyPath)
		fmt.Fprintln(cmd.OutOrStdout(), "ca:  ", caPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tlsCmd)
	tlsCmd.Flags().BoolVar(&tlsVerifyOnly, "verify-only", false, "verify existing files without generating")
}

func verifyKeyPair(certPath, keyPath, caPath string) error {
	if _, err := os.Stat(certPath); err != nil {
		return fmt.Errorf("missing cert %q: %w", certPath, err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("missing key %q: %w", keyPath, err)
	}

	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("load keypair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return errors.New("empty certificate chain")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse leaf: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("leaf certificate not valid before %s", leaf.NotBefore.Format(time.RFC3339))
	}
	if leaf.IsCA {
		return errors.New("leaf certificate unexpectedly marked as CA")
	}
	if time.Until(leaf.NotAfter) <= 0 {
		return fmt.Errorf("leaf certificate expired at %s", leaf.NotAfter.Format(time.RFC3339))
	}
	if leaf.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment) == 0 {
		return fmt.Errorf("unexpected leaf KeyUsage: %v", leaf.KeyUsage)
	}
	if len(leaf.ExtKeyUsage) == 0 {
		return errors.New("leaf ExtKeyUsage is empty")
	}
	serverAuth := false
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			serverAuth = true
			break
		}
	}
	if !serverAuth {
		return errors.New("leaf ExtKeyUsage does not include ServerAuth")
	}

	// If a CA file exists, ensure the leaf verifies against it.
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read CA pem: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return errors.New("failed to parse CA pem")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		return fmt.Errorf("leaf does not verify against CA: %w", err)
	}

	return nil
}
