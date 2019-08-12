// +build !remoteclient

package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo"
)

func SkipIfRemote() {
}

func SkipIfRootless() {
	if os.Geteuid() != 0 {
		ginkgo.Skip("This function is not enabled for rootless podman")
	}
}

func SkipIfNotRunc() {
	runtime := os.Getenv("OCI_RUNTIME")
	if runtime != "" && filepath.Base(runtime) != "runc" {
		ginkgo.Skip("Not using runc as runtime")
	}
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, false, false)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanNoCache calls the podman command with no configured imagecache
func (p *PodmanTestIntegration) PodmanNoCache(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, true, false)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanNoEvents calls the Podman command without an imagecache and without an
// events backend. It is used mostly for caching and uncaching images.
func (p *PodmanTestIntegration) PodmanNoEvents(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, true, true)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanAsUser is the exec call to podman on the filesystem with the specified uid/gid and environment
func (p *PodmanTestIntegration) PodmanAsUser(args []string, uid, gid uint32, cwd string, env []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanAsUserBase(args, uid, gid, cwd, env, false, false)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := filepath.Join(INTEGRATION_ROOT, "test/registries.conf")
	os.Setenv("REGISTRIES_CONFIG_PATH", defaultFile)
}

func (p *PodmanTestIntegration) setRegistriesConfigEnv(b []byte) {
	outfile := filepath.Join(p.TempDir, "registries.conf")
	os.Setenv("REGISTRIES_CONFIG_PATH", outfile)
	ioutil.WriteFile(outfile, b, 0644)
}

func resetRegistriesConfigEnv() {
	os.Setenv("REGISTRIES_CONFIG_PATH", "")
}

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	return PodmanTestCreateUtil(tempDir, false)
}

// MakeOptions assembles all the podman main options
func (p *PodmanTestIntegration) makeOptions(args []string, noEvents bool) []string {
	var debug string
	if _, ok := os.LookupEnv("DEBUG"); ok {
		debug = "--log-level=debug --syslog=true "
	}

	eventsType := "file"
	if noEvents {
		eventsType = "none"
	}

	podmanOptions := strings.Split(fmt.Sprintf("%s--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s --cgroup-manager %s --tmpdir %s --events-backend %s",
		debug, p.CrioRoot, p.RunRoot, p.OCIRuntime, p.ConmonBinary, p.CNIConfigDir, p.CgroupManager, p.TmpDir, eventsType), " ")
	if os.Getenv("HOOK_OPTION") != "" {
		podmanOptions = append(podmanOptions, os.Getenv("HOOK_OPTION"))
	}

	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	fmt.Printf("Restoring %s...\n", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	restore := p.PodmanNoEvents([]string{"load", "-q", "-i", destName})
	restore.Wait(90)
	return nil
}

// RestoreArtifactToCache populates the imagecache from tarballs that were cached earlier
func (p *PodmanTestIntegration) RestoreArtifactToCache(image string) error {
	fmt.Printf("Restoring %s...\n", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))

	p.CrioRoot = p.ImageCacheDir
	restore := p.PodmanNoEvents([]string{"load", "-q", "-i", destName})
	restore.WaitWithDefaultTimeout()
	return nil
}

func (p *PodmanTestIntegration) StopVarlink()     {}
func (p *PodmanTestIntegration) DelayForVarlink() {}

func populateCache(podman *PodmanTestIntegration) {
	for _, image := range CACHE_IMAGES {
		podman.RestoreArtifactToCache(image)
	}
}

func removeCache() {
	// Remove cache dirs
	if err := os.RemoveAll(ImageCacheDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// SeedImages is a no-op for localized testing
func (p *PodmanTestIntegration) SeedImages() error {
	return nil
}
