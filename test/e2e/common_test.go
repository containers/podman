package integration

import (
	"encoding/json"
	"fmt"
	"github.com/containers/libpod/pkg/rootless"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/containers/storage"

	"github.com/containers/libpod/pkg/inspect"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	PODMAN_BINARY    string
	CONMON_BINARY    string
	CNI_CONFIG_DIR   string
	RUNC_BINARY      string
	INTEGRATION_ROOT string
	CGROUP_MANAGER   = "systemd"
	ARTIFACT_DIR     = "/tmp/.artifacts"
	RESTORE_IMAGES   = []string{ALPINE, BB}
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
	Timings             []string
	TmpDir              string
}

var LockTmpDir string

// PodmanSessionIntegration sturct for command line session
type PodmanSessionIntegration struct {
	*PodmanSession
}

type testResult struct {
	name   string
	length float64
}

type testResultsSorted []testResult

func (a testResultsSorted) Len() int      { return len(a) }
func (a testResultsSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type testResultsSortedLength struct{ testResultsSorted }

func (a testResultsSorted) Less(i, j int) bool { return a[i].length < a[j].length }

var testResults []testResult

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

var _ = SynchronizedBeforeSuite(func() []byte {
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
	path, err := ioutil.TempDir("", "libpodlock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return []byte(path)
}, func(data []byte) {
	LockTmpDir = string(data)
})

func (p *PodmanTestIntegration) Setup() {
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	p.ArtifactPath = ARTIFACT_DIR
}

//var _ = BeforeSuite(func() {
//	cwd, _ := os.Getwd()
//	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
//	podman := PodmanTestCreate("/tmp")
//	podman.ArtifactPath = ARTIFACT_DIR
//	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
//		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
//			fmt.Printf("%q\n", err)
//			os.Exit(1)
//		}
//	}
//})
// for _, image := range CACHE_IMAGES {
// 	if err := podman.CreateArtifact(image); err != nil {
// 		fmt.Printf("%q\n", err)
// 		os.Exit(1)
// 	}
// }
// host := GetHostDistributionInfo()
// if host.Distribution == "rhel" && strings.HasPrefix(host.Version, "7") {
// 	f, err := os.OpenFile("/proc/sys/user/max_user_namespaces", os.O_WRONLY, 0644)
// 	if err != nil {
// 		fmt.Println("Unable to enable userspace on RHEL 7")
// 		os.Exit(1)
// 	}
// 	_, err = f.WriteString("15000")
// 	if err != nil {
// 		fmt.Println("Unable to enable userspace on RHEL 7")
// 		os.Exit(1)
// 	}
// 	f.Close()
// }
// path, err := ioutil.TempDir("", "libpodlock")
// if err != nil {
// 	fmt.Println(err)
// 	os.Exit(1)
// }
// LockTmpDir = path
//})

var _ = AfterSuite(func() {
	sort.Sort(testResultsSortedLength{testResults})
	fmt.Println("integration timing results")
	for _, result := range testResults {
		fmt.Printf("%s\t\t%f\n", result.name, result.length)
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
	if rootless.IsRootless() {
		cgroupManager = "cgroupfs"
	}
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
	os.Setenv("DISABLE_HC_SYSTEMD", "true")
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
		TmpDir:              tempDir,
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

// InspectContainer returns a container's inspect data in JSON format
func (p *PodmanTestIntegration) InspectContainer(name string) []inspect.ContainerData {
	cmd := []string{"inspect", name}
	session := p.Podman(cmd)
	session.WaitWithDefaultTimeout()
	return session.InspectContainerToJSON()
}

func processTestResult(f GinkgoTestDescription) {
	tr := testResult{length: f.Duration.Seconds(), name: f.TestText}
	testResults = append(testResults, tr)
}

func GetPortLock(port string) storage.Locker {
	lockFile := filepath.Join(LockTmpDir, port)
	lock, err := storage.GetLockfile(lockFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	lock.Lock()
	return lock
}
