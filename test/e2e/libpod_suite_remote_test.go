//go:build remote_testing && (linux || freebsd)

package integration

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func IsRemote() bool {
	return true
}

// Podman executes podman on the filesystem with default options.
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	return p.PodmanWithOptions(PodmanExecOptions{}, args...)
}

// PodmanWithOptions executes podman on the filesystem with the supplied options.
func (p *PodmanTestIntegration) PodmanWithOptions(options PodmanExecOptions, args ...string) *PodmanSessionIntegration {
	args = p.makeOptions(args, options)
	podmanSession := p.PodmanExecBaseWithOptions(args, options)
	return &PodmanSessionIntegration{podmanSession}
}

func (p *PodmanTestIntegration) setDefaultRegistriesConfigEnv() {
	defaultFile := "registries.conf"
	if UsingCacheRegistry() {
		defaultFile = "registries-cached.conf"
	}
	defaultPath := filepath.Join(INTEGRATION_ROOT, "test", defaultFile)
	os.Setenv("CONTAINERS_REGISTRIES_CONF", defaultPath)
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
	if !isRootless() {
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
	command.Stdout = GinkgoWriter
	command.Stderr = GinkgoWriter
	GinkgoWriter.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	err := command.Start()
	Expect(err).ToNot(HaveOccurred())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.RemoteCommand = command
	p.RemoteSession = command.Process
	err = p.DelayForService()
	Expect(err).ToNot(HaveOccurred())
}

func (p *PodmanTestIntegration) StopRemoteService() {
	if err := p.RemoteSession.Signal(syscall.SIGTERM); err != nil {
		GinkgoWriter.Printf("unable to clean up service %d, %v\n", p.RemoteSession.Pid, err)
	}
	if _, err := p.RemoteSession.Wait(); err != nil {
		GinkgoWriter.Printf("error on remote stop-wait %q", err)
	}
	socket := strings.Split(p.RemoteSocket, ":")[1]
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		GinkgoWriter.Printf("%v\n", err)
	}
	if p.RemoteSocketLock != "" {
		if err := os.Remove(p.RemoteSocketLock); err != nil && !errors.Is(err, os.ErrNotExist) {
			GinkgoWriter.Printf("%v\n", err)
		}
	}
}

// getRemoteOptions assembles all the podman main options
func getRemoteOptions(p *PodmanTestIntegration, args []string) []string {
	networkDir := p.NetworkConfigDir
	podmanOptions := strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --network-config-dir %s --network-backend %s --cgroup-manager %s --tmpdir %s --events-backend %s --db-backend %s",
		p.Root, p.RunRoot, p.OCIRuntime, p.ConmonBinary, networkDir, p.NetworkBackend.ToString(), p.CgroupManager, p.TmpDir, "file", p.DatabaseBackend), " ")
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	tarball := imageTarPath(image)
	if _, err := os.Stat(tarball); err == nil {
		GinkgoWriter.Printf("Restoring %s...\n", image)
		args := []string{"load", "-q", "-i", tarball}
		podmanOptions := getRemoteOptions(p, args)
		command := exec.Command(p.PodmanBinary, podmanOptions...)
		GinkgoWriter.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
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
	var err error
	var conn net.Conn
	for i := 0; i < 100; i++ {
		conn, err = net.Dial("unix", strings.TrimPrefix(p.RemoteSocket, "unix:"))
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("service socket not detected, timeout after 10 seconds: %w", err)
}
