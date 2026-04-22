package certificates

import (
	"crypto/x509"

	"github.com/sirupsen/logrus"
)

// extractHostCertificates retrieved Linux trusted CA certificates from the OS
// trust stores.
func extractHostCertificates() []*x509.Certificate {
	// On Linux, system CA certificates are typically stored in:
	// - /etc/ssl/certs/ca-certificates.crt (Debian/Ubuntu)
	// - /etc/pki/tls/certs/ca-bundle.crt (RHEL/Fedora/CentOS)
	// - /etc/ssl/ca-bundle.pem (OpenSUSE)
	// - /etc/pki/tls/cacert.pem (OpenELEC)
	// - /etc/ssl/cert.pem (Alpine Linux)
	logrus.Warn("extracting trusted CA certificates on Linux is not implemented yet")
	return nil
}
