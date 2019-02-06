package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/libpod/pkg/inspect"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	PODMAN_BINARY      string
	CONMON_BINARY      string
	CNI_CONFIG_DIR     string
	RUNC_BINARY        string
	INTEGRATION_ROOT   string
	CGROUP_MANAGER     = "systemd"
	ARTIFACT_DIR       = "/tmp/.artifacts"
	RESTORE_IMAGES     = []string{ALPINE, BB}
	defaultWaitTimeout = 90
)

// PodmanTestIntegration struct for command line options
type PodmanTestIntegration struct {
	PodmanTest
	ConmonBinary        string
	CrioRoot            string
	CNIConfigDir        string
	OCIRuntime          string
	RunRoot             string
	StorageOptions      string
	SignaturePolicyPath string
	CgroupManager       string
	Host                HostOS
}

// PodmanSessionIntegration sturct for command line session
type PodmanSessionIntegration struct {
	*PodmanSession
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	if reexec.Init() {
		os.Exit(1)
	}
	if os.Getenv("NOCACHE") == "1" {
		CACHE_IMAGES = []string{}
		RESTORE_IMAGES = []string{}
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var _ = BeforeSuite(func() {
	//Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := PodmanTestCreate("/tmp")
	podman.ArtifactPath = ARTIFACT_DIR
	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}
	for _, image := range CACHE_IMAGES {
		if err := podman.CreateArtifact(image); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}
	host := GetHostDistributionInfo()
	if host.Distribution == "rhel" && strings.HasPrefix(host.Version, "7") {
		f, err := os.OpenFile("/proc/sys/user/max_user_namespaces", os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Unable to enable userspace on RHEL 7")
			os.Exit(1)
		}
		_, err = f.WriteString("15000")
		if err != nil {
			fmt.Println("Unable to enable userspace on RHEL 7")
			os.Exit(1)
		}
		f.Close()
	}
})

// PodmanTestCreate creates a PodmanTestIntegration instance for the tests
func PodmanTestCreateUtil(tempDir string, remote bool) *PodmanTestIntegration {
	var (
		podmanRemoteBinary string
	)

	host := GetHostDistributionInfo()
	cwd, _ := os.Getwd()

	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}

	if remote {
		podmanRemoteBinary = filepath.Join(cwd, "../../bin/podman-remote")
		if os.Getenv("PODMAN_REMOTE_BINARY") != "" {
			podmanRemoteBinary = os.Getenv("PODMAN_REMOTE_BINARY")
		}
	}
	conmonBinary := filepath.Join("/usr/libexec/podman/conmon")
	altConmonBinary := "/usr/libexec/crio/conmon"
	if _, err := os.Stat(conmonBinary); os.IsNotExist(err) {
		conmonBinary = altConmonBinary
	}
	if os.Getenv("CONMON_BINARY") != "" {
		conmonBinary = os.Getenv("CONMON_BINARY")
	}
	storageOptions := STORAGE_OPTIONS
	if os.Getenv("STORAGE_OPTIONS") != "" {
		storageOptions = os.Getenv("STORAGE_OPTIONS")
	}
	cgroupManager := CGROUP_MANAGER
	if os.Getenv("CGROUP_MANAGER") != "" {
		cgroupManager = os.Getenv("CGROUP_MANAGER")
	}

	// Ubuntu doesn't use systemd cgroups
	if host.Distribution == "ubuntu" {
		cgroupManager = "cgroupfs"
	}

	ociRuntime := os.Getenv("OCI_RUNTIME")
	if ociRuntime == "" {
		var err error
		ociRuntime, err = exec.LookPath("runc")
		// If we cannot find the runc binary, setting to something static as we have no way
		// to return an error.  The tests will fail and point out that the runc binary could
		// not be found nicely.
		if err != nil {
			ociRuntime = "/usr/bin/runc"
		}
	}

	CNIConfigDir := "/etc/cni/net.d"

	p := &PodmanTestIntegration{
		PodmanTest: PodmanTest{
			PodmanBinary: podmanBinary,
			ArtifactPath: ARTIFACT_DIR,
			TempDir:      tempDir,
			RemoteTest:   remote,
		},
		ConmonBinary:        conmonBinary,
		CrioRoot:            filepath.Join(tempDir, "crio"),
		CNIConfigDir:        CNIConfigDir,
		OCIRuntime:          ociRuntime,
		RunRoot:             filepath.Join(tempDir, "crio-run"),
		StorageOptions:      storageOptions,
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		CgroupManager:       cgroupManager,
		Host:                host,
	}
	if remote {
		p.PodmanTest.RemotePodmanBinary = podmanRemoteBinary
	}

	// Setup registries.conf ENV variable
	p.setDefaultRegistriesConfigEnv()
	// Rewrite the PodmanAsUser function
	p.PodmanMakeOptions = p.makeOptions
	return p
}

// RestoreAllArtifacts unpacks all cached images
func (p *PodmanTestIntegration) RestoreAllArtifacts() error {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return nil
	}
	for _, image := range RESTORE_IMAGES {
		if err := p.RestoreArtifact(image); err != nil {
			return err
		}
	}
	return nil
}

// CreateArtifact creates a cached image in the artifact dir
func (p *PodmanTestIntegration) CreateArtifact(image string) error {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return nil
	}
	fmt.Printf("Caching %s...", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	if _, err := os.Stat(destName); os.IsNotExist(err) {
		pull := p.Podman([]string{"pull", image})
		pull.Wait(90)

		save := p.Podman([]string{"save", "-o", destName, image})
		save.Wait(90)
		fmt.Printf("\n")
	} else {
		fmt.Printf(" already exists.\n")
	}
	return nil
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSessionIntegration) InspectImageJSON() []inspect.ImageData {
	var i []inspect.ImageData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}
