//go:build remote_testing
// +build remote_testing

package integration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/podman/v4/pkg/rootless"
	. "github.com/onsi/gomega"
)

func IsRemote() bool {
	return true
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	args = p.makeOptions(args, false, false)
	podmanSession := p.PodmanBase(args, false, false)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanSystemdScope runs the podman command in a new systemd scope
func (p *PodmanTestIntegration) PodmanSystemdScope(args []string) *PodmanSessionIntegration {
	args = p.makeOptions(args, false, false)

	wrapper := []string{"systemd-run", "--scope"}
	if rootless.IsRootless() {
		wrapper = []string{"systemd-run", "--scope", "--user"}
	}

	podmanSession := p.PodmanAsUserBase(args, 0, 0, "", nil, false, false, wrapper, nil)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanExtraFiles is the exec call to podman on the filesystem and passes down extra files
func (p *PodmanTestIntegration) PodmanExtraFiles(args []string, extraFiles []*os.File) *PodmanSessionIntegration {
	args = p.makeOptions(args, false, false)
	podmanSession := p.PodmanAsUserBase(args, 0, 0, "", nil, false, false, nil, extraFiles)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := filepath.Join(INTEGRATION_ROOT, "test/registries.conf")
	os.Setenv("CONTAINERS_REGISTRIES_CONF", defaultFile)
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
	pti := PodmanTestCreateUtil(tempDir, true)
	pti.StartRemoteService()
	return pti
}

func (p *PodmanTestIntegration) StartRemoteService() {
	if os.Geteuid() == 0 {
		err := os.MkdirAll("/run/podman", 0755)
		Expect(err).ToNot(HaveOccurred())
	}

	args := []string{}
	if _, found := os.LookupEnv("DEBUG_SERVICE"); found {
		args = append(args, "--log-level", "trace")
	}
	remoteSocket := p.RemoteSocket
	args = append(args, "system", "service", "--time", "0", remoteSocket)
	podmanOptions := getRemoteOptions(p, args)
	cacheOptions := []string{"--storage-opt",
		fmt.Sprintf("%s.imagestore=%s", p.PodmanTest.ImageCacheFS, p.PodmanTest.ImageCacheDir)}
	podmanOptions = append(cacheOptions, podmanOptions...)
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	err := command.Start()
	Expect(err).ToNot(HaveOccurred())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.RemoteCommand = command
	p.RemoteSession = command.Process
	p.RemoteStartErr = p.DelayForService()
}

func (p *PodmanTestIntegration) StopRemoteService() {
	if !rootless.IsRootless() {
		if err := p.RemoteSession.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "error on remote stop-kill %q", err)
		}
		if _, err := p.RemoteSession.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "error on remote stop-wait %q", err)
		}
	} else {
		// Stop any children of `podman system service`
		pkill := exec.Command("pkill", "-P", strconv.Itoa(p.RemoteSession.Pid), "-15")
		if err := pkill.Run(); err != nil {
			exitErr := err.(*exec.ExitError)
			if exitErr.ExitCode() != 1 {
				fmt.Fprintf(os.Stderr, "pkill unable to clean up service %d children, exit code %d\n",
					p.RemoteSession.Pid, exitErr.ExitCode())
			}
		}
		if err := p.RemoteSession.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "unable to clean up service %d, %v\n", p.RemoteSession.Pid, err)
		}
	}

	socket := strings.Split(p.RemoteSocket, ":")[1]
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	if p.RemoteSocketLock != "" {
		if err := os.Remove(p.RemoteSocketLock); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
}

// MakeOptions assembles all the podman main options
func getRemoteOptions(p *PodmanTestIntegration, args []string) []string {
	networkDir := p.NetworkConfigDir
	podmanOptions := strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --network-config-dir %s --network-backend %s --cgroup-manager %s",
		p.Root, p.RunRoot, p.OCIRuntime, p.ConmonBinary, networkDir, p.NetworkBackend.ToString(), p.CgroupManager), " ")
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	tarball := imageTarPath(image)
	if _, err := os.Stat(tarball); err == nil {
		fmt.Printf("Restoring %s...\n", image)
		args := []string{"load", "-q", "-i", tarball}
		podmanOptions := getRemoteOptions(p, args)
		command := exec.Command(p.PodmanBinary, podmanOptions...)
		fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
		if err := command.Start(); err != nil {
			return err
		}
		if err := command.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PodmanTestIntegration) DelayForService() error {
	var session *PodmanSessionIntegration
	for i := 0; i < 5; i++ {
		session = p.Podman([]string{"info"})
		session.WaitWithDefaultTimeout()
		if session.ExitCode() == 0 {
			return nil
		} else if i == 4 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("service not detected, exit code(%d)", session.ExitCode())
}
