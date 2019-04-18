// +build remoteclient

package integration

import (
	"fmt"
	"github.com/containers/libpod/pkg/rootless"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo"
)

func SkipIfRemote() {
	ginkgo.Skip("This function is not enabled for remote podman")
}

func SkipIfRootless() {
	if os.Geteuid() != 0 {
		ginkgo.Skip("This function is not enabled for remote podman")
	}
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args)
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
	pti := PodmanTestCreateUtil(tempDir, true)
	pti.StartVarlink()
	return pti
}

func (p *PodmanTestIntegration) StartVarlink() {
	if os.Geteuid() == 0 {
		os.MkdirAll("/run/podman", 0755)
	}
	varlinkEndpoint := p.VarlinkEndpoint
	if addr := os.Getenv("PODMAN_VARLINK_ADDRESS"); addr != "" {
		varlinkEndpoint = addr
	}

	args := []string{"varlink", "--timeout", "0", varlinkEndpoint}
	podmanOptions := getVarlinkOptions(p, args)
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command.Start()
	p.VarlinkSession = command.Process
}

func (p *PodmanTestIntegration) StopVarlink() {
	varlinkSession := p.VarlinkSession
	varlinkSession.Kill()
	varlinkSession.Wait()

	if !rootless.IsRootless() {
		socket := strings.Split(p.VarlinkEndpoint, ":")[1]
		if err := os.Remove(socket); err != nil {
			fmt.Println(err)
		}
	}
}

//MakeOptions assembles all the podman main options
func (p *PodmanTestIntegration) makeOptions(args []string) []string {
	return args
}

//MakeOptions assembles all the podman main options
func getVarlinkOptions(p *PodmanTestIntegration, args []string) []string {
	podmanOptions := strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s --cgroup-manager %s",
		p.CrioRoot, p.RunRoot, p.OCIRuntime, p.ConmonBinary, p.CNIConfigDir, p.CgroupManager), " ")
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
	args := []string{"load", "-q", "-i", destName}
	podmanOptions := getVarlinkOptions(p, args)
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command.Start()
	command.Wait()
	return nil
}
