package certificates

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func verifyCertificateFile(filePath string) error {
	cmd := exec.Command(
		`powershell`,
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"(Get-PfxCertificate -FilePath '"+filePath+"').Verify()",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("output: %s, error: %s", out, err)
	}
	if !strings.Contains(string(out), "True") {
		return fmt.Errorf("certificate verification failed")
	}
	return nil
}

func TestExtractAndSaveCertificates(t *testing.T) {
	certs := extractHostCertificates()
	assert.NotEmpty(t, certs)

	filePath := filepath.Join(t.TempDir(), "cert.pem")
	err := saveCertificatesToPEM(certs, filePath)
	assert.NoError(t, err)

	err = verifyCertificateFile(filePath)
	assert.NoError(t, err)
}
