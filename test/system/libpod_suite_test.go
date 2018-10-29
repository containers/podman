package system

import (
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	PODMAN_BINARY string
	GLOBALOPTIONS = []string{"--cgroup-manager",
		"--cni-config-dir",
		"--config", "-c",
		"--conmon",
		"--cpu-profile",
		"--log-level",
		"--root",
		"--tmpdir",
		"--runroot",
		"--runtime",
		"--storage-driver",
		"--storage-opt",
		"--syslog",
	}
	PODMAN_SUBCMD = []string{"attach",
		"commit",
		"container",
		"build",
		"create",
		"diff",
		"exec",
		"export",
		"history",
		"image",
		"images",
		"import",
		"info",
		"inspect",
		"kill",
		"load",
		"login",
		"logout",
		"logs",
		"mount",
		"pause",
		"ps",
		"pod",
		"port",
		"pull",
		"push",
		"restart",
		"rm",
		"rmi",
		"run",
		"save",
		"search",
		"start",
		"stats",
		"stop",
		"tag",
		"top",
		"umount",
		"unpause",
		"version",
		"wait",
		"h",
	}
	INTEGRATION_ROOT   string
	ARTIFACT_DIR       = "/tmp/.artifacts"
	ALPINE             = "docker.io/library/alpine:latest"
	BB                 = "docker.io/library/busybox:latest"
	BB_GLIBC           = "docker.io/library/busybox:glibc"
	fedoraMinimal      = "registry.fedoraproject.org/fedora-minimal:latest"
	nginx              = "quay.io/baude/alpine_nginx:latest"
	redis              = "docker.io/library/redis:alpine"
	registry           = "docker.io/library/registry:2"
	infra              = "k8s.gcr.io/pause:3.1"
	defaultWaitTimeout = 90
)

// PodmanTestSystem struct for command line options
type PodmanTestSystem struct {
	PodmanTest
	GlobalOptions    map[string]string
	PodmanCmdOptions map[string][]string
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var _ = BeforeSuite(func() {
})

// PodmanTestCreate creates a PodmanTestSystem instance for the tests
func PodmanTestCreate(tempDir string) *PodmanTestSystem {
	var envKey string
	globalOptions := make(map[string]string)
	podmanCmdOptions := make(map[string][]string)

	for _, n := range GLOBALOPTIONS {
		envKey = strings.Replace(strings.ToUpper(strings.Trim(n, "-")), "-", "_", -1)
		if isEnvSet(envKey) {
			globalOptions[n] = os.Getenv(envKey)
		}
	}

	for _, n := range PODMAN_SUBCMD {
		envKey = strings.Replace("PODMAN_SUBCMD_OPTIONS", "SUBCMD", strings.ToUpper(n), -1)
		if isEnvSet(envKey) {
			podmanCmdOptions[n] = strings.Split(os.Getenv(envKey), " ")
		}
	}

	podmanBinary := "podman"
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}

	p := &PodmanTestSystem{
		PodmanTest: PodmanTest{
			PodmanBinary: podmanBinary,
			ArtifactPath: ARTIFACT_DIR,
			TempDir:      tempDir,
		},
		GlobalOptions:    globalOptions,
		PodmanCmdOptions: podmanCmdOptions,
	}

	p.PodmanMakeOptions = p.makeOptions

	return p
}

func (p *PodmanTestSystem) Podman(args []string) *PodmanSession {
	return p.PodmanBase(args)
}

//MakeOptions assembles all the podman options
func (p *PodmanTestSystem) makeOptions(args []string) []string {
	var addOptions, subArgs []string
	for _, n := range GLOBALOPTIONS {
		if p.GlobalOptions[n] != "" {
			addOptions = append(addOptions, n, p.GlobalOptions[n])
		}
	}

	if len(args) == 0 {
		return addOptions
	}

	subCmd := args[0]
	addOptions = append(addOptions, subCmd)
	if subCmd == "unmount" {
		subCmd = "umount"
	}
	if subCmd == "help" {
		subCmd = "h"
	}

	if _, ok := p.PodmanCmdOptions[subCmd]; ok {
		m := make(map[string]bool)
		subArgs = p.PodmanCmdOptions[subCmd]
		for i := 0; i < len(subArgs); i++ {
			m[subArgs[i]] = true
		}
		for i := 1; i < len(args); i++ {
			if _, ok := m[args[i]]; !ok {
				subArgs = append(subArgs, args[i])
			}
		}
	} else {
		subArgs = args[1:]
	}

	addOptions = append(addOptions, subArgs...)

	return addOptions
}

// Cleanup cleans up the temporary store
func (p *PodmanTestSystem) Cleanup() {
	// Remove all containers
	stopall := p.Podman([]string{"stop", "-a", "--timeout", "0"})
	stopall.WaitWithDefaultTimeout()

	session := p.Podman([]string{"rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// CleanupPod cleans up the temporary store
func (p *PodmanTestSystem) CleanupPod() {
	// Remove all containers
	session := p.Podman([]string{"pod", "rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// Check if the key is set in Env
func isEnvSet(key string) bool {
	_, set := os.LookupEnv(key)
	return set
}
