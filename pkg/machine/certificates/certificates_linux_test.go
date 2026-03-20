package certificates

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func verifyCertificateFile(_ string) error {
	return nil
}

func TestExtractAndSaveCertificates(t *testing.T) {
	certs := extractHostCertificates()
	// On Linux extractHostCertificates is not implemented
	// therefore `certs` is expected to be empty.
	assert.Empty(t, certs)

	filePath := filepath.Join(t.TempDir(), "cert.pem")
	err := saveCertificatesToPEM(certs, filePath)
	assert.NoError(t, err)

	err = verifyCertificateFile(filePath)
	assert.NoError(t, err)
}
