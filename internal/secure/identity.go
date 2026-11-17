package secure

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Identity is one Ed25519 keypair. The private half never leaves the keyring.
type Identity struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

func newIdentity() (Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, fmt.Errorf("generate identity: %w", err)
	}
	return Identity{Private: priv, Public: pub}, nil
}

func encodeIdentity(i Identity) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(i.Private)
	if err != nil {
		return "", fmt.Errorf("encode identity: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(der), nil
}

func parseIdentity(secret string) (Identity, error) {
	der, err := base64.RawStdEncoding.DecodeString(secret)
	if err != nil {
		return Identity{}, errors.New("the stored identity is damaged")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return Identity{}, errors.New("the stored identity is damaged")
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok || len(priv) != ed25519.PrivateKeySize {
		return Identity{}, errors.New("the stored identity is not an Ed25519 key")
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return Identity{}, errors.New("the stored identity has no public key")
	}
	return Identity{Private: priv, Public: pub}, nil
}

// Certificate builds a self-signed certificate from the identity. The
// certificate is not trusted by a CA; the other side pins the public key.
func (i Identity) Certificate(usage x509.ExtKeyUsage) (tls.Certificate, error) {
	if len(i.Private) != ed25519.PrivateKeySize || len(i.Public) != ed25519.PublicKeySize {
		return tls.Certificate{}, errors.New("the identity is incomplete")
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate serial: %w", err)
	}

	name := "LAN Party device"
	if usage == x509.ExtKeyUsageServerAuth {
		name = "LAN Party host"
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{usage},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, i.Public, i.Private)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  i.Private,
		Leaf:        leaf,
	}, nil
}

// Pin is the stable public-key fingerprint friends store in a group. It
// survives certificate renewal because it covers the key, not the document.
func Pin(pub ed25519.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// PinMatches compares a public key with a stored pin.
func PinMatches(pub ed25519.PublicKey, pin string) bool {
	got := Pin(pub)
	if got == "" || pin == "" {
		return false
	}
	return got == pin
}

// Fingerprint is the short form shown in the interface. It is a display aid,
// not the value used for verification.
func Fingerprint(pub ed25519.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	text := strings.ToUpper(hex.EncodeToString(sum[:8]))
	return text[0:4] + "-" + text[4:8] + "-" + text[8:12] + "-" + text[12:16]
}

// EncodePublicKey is the form stored in group JSON. It is public data.
func EncodePublicKey(pub ed25519.PublicKey) string {
	return base64.RawStdEncoding.EncodeToString(pub)
}

// DecodePublicKey reads a public key written by EncodePublicKey.
func DecodePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key")
	}
	return ed25519.PublicKey(raw), nil
}

// PublicKeyFromCertificate returns the Ed25519 key carried by a certificate.
func PublicKeyFromCertificate(cert *x509.Certificate) (ed25519.PublicKey, error) {
	if cert == nil {
		return nil, errors.New("missing certificate")
	}
	pub, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("the certificate does not carry an Ed25519 key")
	}
	return pub, nil
}
