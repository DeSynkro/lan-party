package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const certificateLifetime = 10 * 365 * 24 * time.Hour

// GenerateIdentity creates a relay key and returns its TLS certificate and
// public-key fingerprint.
func GenerateIdentity(keyPath string) (tls.Certificate, string, error) {
	if _, err := os.Stat(keyPath); err == nil {
		return tls.Certificate{}, "", fmt.Errorf("relay key already exists at %s", keyPath)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("generate relay key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("encode relay key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("create relay key directory: %w", err)
	}
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("create relay key: %w", err)
	}
	if _, err := file.Write(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err != nil {
		_ = file.Close()
		_ = os.Remove(keyPath)
		return tls.Certificate{}, "", fmt.Errorf("write relay key: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(keyPath)
		return tls.Certificate{}, "", fmt.Errorf("close relay key: %w", err)
	}

	cert, err := certificateFor(pub, priv)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return cert, publicKeyFingerprint(cert.Certificate[0]), nil
}

// LoadIdentity reads a relay key and builds its TLS certificate.
func LoadIdentity(keyPath string) (tls.Certificate, string, error) {
	if err := checkSecretFile(keyPath); err != nil {
		return tls.Certificate{}, "", err
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("read relay key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return tls.Certificate{}, "", errors.New("relay key is not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("parse relay key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return tls.Certificate{}, "", errors.New("relay key is not Ed25519")
	}
	pub := priv.Public().(ed25519.PublicKey)
	cert, err := certificateFor(pub, priv)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return cert, publicKeyFingerprint(cert.Certificate[0]), nil
}

func certificateFor(pub ed25519.PublicKey, priv ed25519.PrivateKey) (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("make certificate serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "LAN Party Relay"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(certificateLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create relay certificate: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, nil
}

func publicKeyFingerprint(certDER []byte) string {
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return ""
	}
	der, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// ClientTLS returns a TLS configuration that pins the relay public key.
func ClientTLS(fingerprint string) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, // the pinned public key replaces CA and hostname checks
		MinVersion:         tls.VersionTLS13,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("relay sent no certificate")
			}
			if got := publicKeyFingerprint(rawCerts[0]); got != fingerprint {
				return fmt.Errorf("relay key mismatch: got %s", got)
			}
			return nil
		},
	}
}
