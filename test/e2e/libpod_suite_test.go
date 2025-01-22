//go:build !remote_testing && (linux || freebsd)

package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func IsRemote() bool {
	return false
}

// Podman executes podman on the filesystem with default options.
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	return p.PodmanWithOptions(PodmanExecOptions{}, args...)
}

// PodmanWithOptions executes podman on the filesystem with the supplied options.
func (p *PodmanTestIntegration) PodmanWithOptions(options PodmanExecOptions, args ...string) *PodmanSessionIntegration {
	podmanSession := p.PodmanExecBaseWithOptions(args, options)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := "registries.conf"
	if UsingCacheRegistry() {
		defaultFile = "registries-cached.conf"
	}
	defaultPath := filepath.Join(INTEGRATION_ROOT, "test", defaultFile)
	err := os.Setenv("CONTAINERS_REGISTRIES_CONF", defaultPath)
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
