package test_bindings

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"
)

const (
	defaultPodmanBinaryLocation string = "/usr/bin/podman"
	alpine                      string = "docker.io/library/alpine:latest"
	busybox                     string = "docker.io/library/busybox:latest"
)

type bindingTest struct {
	artifactDirPath string
	imageCacheDir   string
	sock            string
	tempDirPath     string
	runRoot         string
	crioRoot        string
}

func (b *bindingTest) runPodman(command []string) *gexec.Session {
	var cmd []string
	podmanBinary := defaultPodmanBinaryLocation
	val, ok := os.LookupEnv("PODMAN_BINARY")
	if ok {
		podmanBinary = val
	}
	val, ok = os.LookupEnv("CGROUP_MANAGER")
	if ok {
		cmd = append(cmd, "--cgroup-manager", val)
	}
	val, ok = os.LookupEnv("CNI_CONFIG_DIR")
	if ok {
		cmd = append(cmd, "--cni-config-dir", val)
	}
	val, ok = os.LookupEnv("CONMON")
	if ok {
		cmd = append(cmd, "--conmon", val)
	}
	val, ok = os.LookupEnv("ROOT")
	if ok {
		cmd = append(cmd, "--root", val)
	} else {
		cmd = append(cmd, "--root", b.crioRoot)
	}
	val, ok = os.LookupEnv("OCI_RUNTIME")
	if ok {
		cmd = append(cmd, "--runtime", val)
	}
	val, ok = os.LookupEnv("RUNROOT")
	if ok {
		cmd = append(cmd, "--runroot", val)
	} else {
		cmd = append(cmd, "--runroot", b.runRoot)
	}
	val, ok = os.LookupEnv("TEMPDIR")
	if ok {
		cmd = append(cmd, "--tmpdir", val)
	} else {
		cmd = append(cmd, "--tmpdir", b.tempDirPath)
	}
	val, ok = os.LookupEnv("STORAGE_DRIVER")
	if ok {
		cmd = append(cmd, "--storage-driver", val)
	}
	val, ok = os.LookupEnv("STORAGE_OPTIONS")
	if ok {
		cmd = append(cmd, "--storage", val)
	}
	cmd = append(cmd, command...)
	c := exec.Command(podmanBinary, cmd...)
	fmt.Printf("Running: %s %s\n", podmanBinary, strings.Join(cmd, " "))
	session, err := gexec.Start(c, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		panic(errors.Errorf("unable to run podman command: %q", cmd))
	}
	return session
}

func newBindingTest() *bindingTest {
	tmpPath, _ := createTempDirInTempDir()
	b := bindingTest{
		crioRoot:        filepath.Join(tmpPath, "crio"),
		runRoot:         filepath.Join(tmpPath, "run"),
		artifactDirPath: "",
		imageCacheDir:   "",
		sock:            fmt.Sprintf("unix:%s", filepath.Join(tmpPath, "api.sock")),
		tempDirPath:     tmpPath,
	}
	return &b
}

// createTempDirinTempDir create a temp dir with prefix podman_test
func createTempDirInTempDir() (string, error) {
	return ioutil.TempDir("", "libpod_api")
}

func (b *bindingTest) startAPIService() *gexec.Session {
	var (
		cmd []string
	)
	cmd = append(cmd, "--log-level=debug", "service", "--timeout=999999", b.sock)
	return b.runPodman(cmd)
}

func (b *bindingTest) cleanup() {
	s := b.runPodman([]string{"stop", "-a", "-t", "0"})
	s.Wait(45)
	if err := os.RemoveAll(b.tempDirPath); err != nil {
		fmt.Println(err)
	}
}

// Pull is a helper function to pull in images
func (b *bindingTest) Pull(name string) {
	p := b.runPodman([]string{"pull", name})
	p.Wait(45)
}

// Run a container and add append the alpine image to it
func (b *bindingTest) RunTopContainer(name *string) {
	cmd := []string{"run", "-dt"}
	if name != nil {
		containerName := *name
		cmd = append(cmd, "--name", containerName)
	}
	cmd = append(cmd, alpine, "top")
	p := b.runPodman(cmd)
	p.Wait(45)
}

//  StringInSlice returns a boolean based on whether a given
//  string is in a given slice
func StringInSlice(s string, sl []string) bool {
	for _, val := range sl {
		if s == val {
			return true
		}
	}
	return false
}
