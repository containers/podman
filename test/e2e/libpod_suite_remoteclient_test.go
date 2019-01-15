// +build remoteclient

package integration

import (
	"fmt"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/onsi/ginkgo"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func SkipIfRemote() {
	ginkgo.Skip("This function is not enabled for remote podman")
}

// Cleanup cleans up the temporary store
func (p *PodmanTestIntegration) Cleanup() {
	p.StopVarlink()
	// TODO
	// Stop all containers
	// Rm all containers

	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args)
	return &PodmanSessionIntegration{podmanSession}
}

//RunTopContainer runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunTopContainer(name string) *PodmanSessionIntegration {
	// TODO
	return nil
}

//RunLsContainer runs a simple container in the background that
// simply runs ls. If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunLsContainer(name string) (*PodmanSessionIntegration, int, string) {
	// TODO
	return nil, 0, ""
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSessionIntegration) InspectImageJSON() []inspect.ImageData {
	// TODO
	return nil
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

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSessionIntegration) InspectContainerToJSON() []inspect.ContainerData {
	// TODO
	return nil
}

// CreatePod creates a pod with no infra container
// it optionally takes a pod name
func (p *PodmanTestIntegration) CreatePod(name string) (*PodmanSessionIntegration, int, string) {
	// TODO
	return nil, 0, ""
}

func (p *PodmanTestIntegration) RunTopContainerInPod(name, pod string) *PodmanSessionIntegration {
	// TODO
	return nil
}

// BuildImage uses podman build and buildah to build an image
// called imageName based on a string dockerfile
func (p *PodmanTestIntegration) BuildImage(dockerfile, imageName string, layers string) {
	// TODO
}

// CleanupPod cleans up the temporary store
func (p *PodmanTestIntegration) CleanupPod() {
	// TODO
}

// InspectPodToJSON takes the sessions output from a pod inspect and returns json
func (s *PodmanSessionIntegration) InspectPodToJSON() libpod.PodInspect {
	// TODO
	return libpod.PodInspect{}
}
func (p *PodmanTestIntegration) RunLsContainerInPod(name, pod string) (*PodmanSessionIntegration, int, string) {
	// TODO
	return nil, 0, ""
}

// PullImages pulls multiple images
func (p *PodmanTestIntegration) PullImages(images []string) error {
	// TODO
	return libpod.ErrNotImplemented
}

// PodmanPID execs podman and returns its PID
func (p *PodmanTestIntegration) PodmanPID(args []string) (*PodmanSessionIntegration, int) {
	// TODO
	return nil, 0
}

// CleanupVolume cleans up the temporary store
func (p *PodmanTestIntegration) CleanupVolume() {
	// TODO
}

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, true)
	pti.StartVarlink()
	return pti
}

func (p *PodmanTestIntegration) StartVarlink() {
	if _, err := os.Stat("/path/to/whatever"); os.IsNotExist(err) {
		os.MkdirAll("/run/podman", 0755)
	}
	args := []string{"varlink", "--timeout", "0", "unix:/run/podman/io.podman"}
	podmanOptions := p.MakeOptions(args)
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command.Start()
	p.VarlinkSession = command.Process
}

func (p *PodmanTestIntegration) StopVarlink() {
	varlinkSession := p.VarlinkSession
	varlinkSession.Kill()
	varlinkSession.Wait()
}
