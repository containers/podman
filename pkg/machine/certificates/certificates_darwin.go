package certificates

import (
	"crypto/x509"

	"github.com/sirupsen/logrus"
)

// extractHostCertificates extracts trusted CA certificates from the macOS system keychain
func extractHostCertificates() []*x509.Certificate {
	logrus.Warn("extracting trusted CA certificates on macOS is not implemented yet")
	return nil
}
