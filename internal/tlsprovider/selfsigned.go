package tlsprovider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"
)

type SelfSignedOptions struct {
	DNSNames []string
	IPs      []net.IP
	ValidFor time.Duration
}

func GenerateSelfSigned(opts SelfSignedOptions) (tls.Certificate, error) {
	certPEM, keyPEM, _, err := GenerateSelfSignedWithCA(opts)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func GenerateSelfSignedWithCA(opts SelfSignedOptions) (certPEM []byte, keyPEM []byte, caPEM []byte, err error) {
	validFor := opts.ValidFor
	if validFor == 0 {
		validFor = 365 * 24 * time.Hour
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	notBefore := time.Now().Add(-5 * time.Minute)

	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	caSerial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, nil, err
	}
	caTmpl := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName:   "gonetsim CA",
			Organization: []string{"gonetsim"},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(validFor),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, err
	}
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serverSerial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, nil, err
	}
	serverTmpl := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName:   "gonetsim",
			Organization: []string{"gonetsim"},
		},
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(validFor),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              append([]string(nil), opts.DNSNames...),
		IPAddresses:           append([]net.IP(nil), opts.IPs...),
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caTmpl, &serverPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, err
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})

	keyBytes, err := x509.MarshalPKCS8PrivateKey(serverPriv)
	if err != nil {
		return nil, nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	certPEM = append(append([]byte(nil), leafPEM...), caPEM...)
	return certPEM, keyPEM, caPEM, nil
}
