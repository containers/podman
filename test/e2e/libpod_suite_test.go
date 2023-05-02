//go:build !remote_testing
// +build !remote_testing

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func IsRemote() bool {
	return false
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, false, false)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanSystemdScope runs the podman command in a new systemd scope
func (p *PodmanTestIntegration) PodmanSystemdScope(args []string) *PodmanSessionIntegration {
	wrapper := []string{"systemd-run", "--scope"}
	if isRootless() {
		wrapper = []string{"systemd-run", "--scope", "--user"}
	}
	podmanSession := p.PodmanAsUserBase(args, 0, 0, "", nil, false, false, wrapper, nil)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanExtraFiles is the exec call to podman on the filesystem and passes down extra files
func (p *PodmanTestIntegration) PodmanExtraFiles(args []string, extraFiles []*os.File) *PodmanSessionIntegration {
	podmanSession := p.PodmanAsUserBase(args, 0, 0, "", nil, false, false, nil, extraFiles)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := filepath.Join(INTEGRATION_ROOT, "test/registries.conf")
	err := os.Setenv("CONTAINERS_REGISTRIES_CONF", defaultFile)
	Expect(err).ToNot(HaveOccurred())
}

func (p *PodmanTestIntegration) setRegistriesConfigEnv(b []byte) {
	outfile := filepath.Join(p.TempDir, "registries.conf")
	os.Setenv("CONTAINERS_REGISTRIES_CONF", outfile)
	err := os.WriteFile(outfile, b, 0644)
	Expect(err).ToNot(HaveOccurred())
}

func resetRegistriesConfigEnv() {
	os.Setenv("CONTAINERS_REGISTRIES_CONF", "")
}

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	return PodmanTestCreateUtil(tempDir, false)
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	tarball := imageTarPath(image)
	if _, err := os.Stat(tarball); err == nil {
		GinkgoWriter.Printf("Restoring %s...\n", image)
		restore := p.PodmanNoEvents([]string{"load", "-q", "-i", tarball})
		restore.Wait(90)
	}
	return nil
}

func (p *PodmanTestIntegration) StopRemoteService() {}

// We don't support running API service when local
func (p *PodmanTestIntegration) StartRemoteService() {
}

// Just a stub for compiling with `!remote`.
func getRemoteOptions(p *PodmanTestIntegration, args []string) []string {
	return nil
}
