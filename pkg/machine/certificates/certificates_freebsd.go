package certificates

import (
	"crypto/x509"

	"github.com/sirupsen/logrus"
)

// extractHostCertificates extracts trusted CA certificates from the FreeBSD system keychain
func extractHostCertificates() []*x509.Certificate {
	logrus.Warn("extracting trusted CA certificates on FreeBSD is not implemented yet")
	return nil
}
