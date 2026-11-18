package tunnel

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

const (
	defaultMaxStreams           = 128
	defaultApprovalTimeout      = 2 * time.Minute
	defaultMaxPendingHandshakes = 32
	defaultHandshakeTimeout     = 10 * time.Second
	controlWriteTimeout         = 5 * time.Second
	denialGrace                 = 250 * time.Millisecond
)

func serverTLS(identity secure.Identity) (*tls.Config, error) {
	cert, err := identity.Certificate(x509.ExtKeyUsageServerAuth)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{ALPN},
	}, nil
}

func clientTLS(identity secure.Identity, hostPin string) (*tls.Config, error) {
	if hostPin == "" {
		return nil, errors.New("the host key pin is missing")
	}
	cert, err := identity.Certificate(x509.ExtKeyUsageClientAuth)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{ALPN},
		// The host certificate is self-signed and its key is pinned in the
		// invite. There is no chain to verify; the pin check below is the
		// authentication.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("the host sent no certificate")
			}
			pub, err := secure.PublicKeyFromCertificate(state.PeerCertificates[0])
			if err != nil {
				return err
			}
			if !secure.PinMatches(pub, hostPin) {
				return errors.New("the host key does not match the invite")
			}
			return nil
		},
	}, nil
}

func serverQUICConfig(maxStreams int) *quic.Config {
	if maxStreams <= 0 {
		maxStreams = defaultMaxStreams
	}
	return &quic.Config{
		HandshakeIdleTimeout:  10 * time.Second,
		MaxIdleTimeout:        30 * time.Second,
		KeepAlivePeriod:       10 * time.Second,
		MaxIncomingStreams:    int64(maxStreams + 1), // control stream plus data
		MaxIncomingUniStreams: -1,
		EnableDatagrams:       true,
	}
}

func clientQUICConfig() *quic.Config {
	return &quic.Config{
		HandshakeIdleTimeout:  10 * time.Second,
		MaxIdleTimeout:        30 * time.Second,
		KeepAlivePeriod:       10 * time.Second,
		MaxIncomingStreams:    -1,
		MaxIncomingUniStreams: -1,
		EnableDatagrams:       true,
	}
}
