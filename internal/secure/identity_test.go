package secure

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"regexp"
	"testing"
)

func testIdentity(t *testing.T) Identity {
	t.Helper()
	identity, err := newIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestCertificateIsSelfSigned(t *testing.T) {
	identity := testIdentity(t)
	usage := x509.ExtKeyUsageServerAuth

	cert, err := identity.Certificate(usage)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Leaf == nil {
		t.Fatal("certificate has no parsed leaf")
	}
	if err := cert.Leaf.CheckSignature(cert.Leaf.SignatureAlgorithm, cert.Leaf.RawTBSCertificate, cert.Leaf.Signature); err != nil {
		t.Fatalf("certificate is not self-signed: %v", err)
	}
	if cert.Leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("certificate is missing digital signature usage")
	}
	if len(cert.Leaf.ExtKeyUsage) != 1 || cert.Leaf.ExtKeyUsage[0] != usage {
		t.Fatalf("extended usage = %v", cert.Leaf.ExtKeyUsage)
	}
	got, ok := cert.Leaf.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(got, identity.Public) {
		t.Fatal("certificate carries a different public key")
	}
}

func TestCertificateRequiresACompleteIdentity(t *testing.T) {
	if _, err := (Identity{}).Certificate(x509.ExtKeyUsageClientAuth); err == nil {
		t.Fatal("an empty identity produced a certificate")
	}
}

func TestPinIsStable(t *testing.T) {
	identity := testIdentity(t)
	pin := Pin(identity.Public)
	if len(pin) != 43 {
		t.Fatalf("pin length = %d", len(pin))
	}
	if Pin(identity.Public) != pin {
		t.Fatal("pin changed between calls")
	}
	if !PinMatches(identity.Public, pin) {
		t.Fatal("pin did not match its key")
	}
	if PinMatches(testIdentity(t).Public, pin) {
		t.Fatal("a different key matched the pin")
	}
}

func TestFingerprintShape(t *testing.T) {
	identity := testIdentity(t)
	got := Fingerprint(identity.Public)
	if !regexp.MustCompile(`^[0-9A-F]{4}(-[0-9A-F]{4}){3}$`).MatchString(got) {
		t.Fatalf("fingerprint = %q", got)
	}
}

func TestPublicKeyFromCertificate(t *testing.T) {
	identity := testIdentity(t)
	cert, err := identity.Certificate(x509.ExtKeyUsageClientAuth)
	if err != nil {
		t.Fatal(err)
	}
	got, err := PublicKeyFromCertificate(cert.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, identity.Public) {
		t.Fatal("public key did not round-trip")
	}
}

func TestPublicKeyFromCertificateRejectsOtherKeys(t *testing.T) {
	if _, err := PublicKeyFromCertificate(nil); err == nil {
		t.Fatal("nil certificate was accepted")
	}
	if _, err := PublicKeyFromCertificate(&x509.Certificate{PublicKey: "not a key"}); err == nil {
		t.Fatal("non-Ed25519 key was accepted")
	}
}
