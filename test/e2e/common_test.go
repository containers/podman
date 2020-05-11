package integration

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/libpod/pkg/rootless"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/stringid"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"
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
	Timings             []string
	TmpDir              string
	RemoteStartErr      error
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

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	if os.Getenv("NOCACHE") == "1" {
		CACHE_IMAGES = []string{}
		RESTORE_IMAGES = []string{}
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Cache images
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

	// make cache dir
	if err := os.MkdirAll(ImageCacheDir, 0777); err != nil {
		fmt.Printf("%q\n", err)
		os.Exit(1)
	}

	for _, image := range CACHE_IMAGES {
		podman.createArtifact(image)
	}

	// If running localized tests, the cache dir is created and populated. if the
	// tests are remote, this is a no-op
	populateCache(podman)

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

var _ = SynchronizedAfterSuite(func() {},
	func() {
		sort.Sort(testResultsSortedLength{testResults})
		fmt.Println("integration timing results")
		for _, result := range testResults {
			fmt.Printf("%s\t\t%f\n", result.name, result.length)
		}

		// previous crio-run
		tempdir, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest := PodmanTestCreate(tempdir)

		if err := os.RemoveAll(podmanTest.CrioRoot); err != nil {
			fmt.Printf("%q\n", err)
		}

		// for localized tests, this removes the image cache dir and for remote tests
		// this is a no-op
		removeCache()
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
	altConmonBinary := "/usr/bin/conmon"
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

	ociRuntime := os.Getenv("OCI_RUNTIME")
	if ociRuntime == "" {
		var err error
		ociRuntime, err = exec.LookPath("crun")
		// If we cannot find the crun binary, setting to something static as we have no way
		// to return an error.  The tests will fail and point out that the runc binary could
		// not be found nicely.
		if err != nil {
			ociRuntime = "/usr/bin/runc"
		}
	}
	os.Setenv("DISABLE_HC_SYSTEMD", "true")
	CNIConfigDir := "/etc/cni/net.d"

	storageFs := STORAGE_FS
	if rootless.IsRootless() {
		storageFs = ROOTLESS_STORAGE_FS
	}
	p := &PodmanTestIntegration{
		PodmanTest: PodmanTest{
			PodmanBinary:  podmanBinary,
			ArtifactPath:  ARTIFACT_DIR,
			TempDir:       tempDir,
			RemoteTest:    remote,
			ImageCacheFS:  storageFs,
			ImageCacheDir: ImageCacheDir,
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
		uuid := stringid.GenerateNonCryptoID()
		if !rootless.IsRootless() {
			p.RemoteSocket = fmt.Sprintf("unix:/run/podman/podman-%s.sock", uuid)
		} else {
			runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
			socket := fmt.Sprintf("podman-%s.sock", uuid)
			fqpath := filepath.Join(runtimeDir, socket)
			p.RemoteSocket = fmt.Sprintf("unix:%s", fqpath)
		}
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

// createArtifact creates a cached image in the artifact dir
func (p *PodmanTestIntegration) createArtifact(image string) {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return
	}
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	fmt.Printf("Caching %s at %s...", image, destName)
	if _, err := os.Stat(destName); os.IsNotExist(err) {
		pull := p.PodmanNoCache([]string{"pull", image})
		pull.Wait(90)
		Expect(pull.ExitCode()).To(Equal(0))

		save := p.PodmanNoCache([]string{"save", "-o", destName, image})
		save.Wait(90)
		Expect(save.ExitCode()).To(Equal(0))
		fmt.Printf("\n")
	} else {
		fmt.Printf(" already exists.\n")
	}
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSessionIntegration) InspectImageJSON() []inspect.ImageData {
	var i []inspect.ImageData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectContainer returns a container's inspect data in JSON format
func (p *PodmanTestIntegration) InspectContainer(name string) []define.InspectContainerData {
	cmd := []string{"inspect", name}
	session := p.Podman(cmd)
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
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

// GetRandomIPAddress returns a random IP address to avoid IP
// collisions during parallel tests
func GetRandomIPAddress() string {
	// To avoid IP collisions of initialize random seed for random IP addresses
	rand.Seed(time.Now().UnixNano())
	// Add GinkgoParallelNode() on top of the IP address
	// in case of the same random seed
	ip3 := strconv.Itoa(rand.Intn(230) + GinkgoParallelNode())
	ip4 := strconv.Itoa(rand.Intn(230) + GinkgoParallelNode())
	return "10.88." + ip3 + "." + ip4
}

// RunTopContainer runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunTopContainer(name string) *PodmanSessionIntegration {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

// RunLsContainer runs a simple container in the background that
// simply runs ls. If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunLsContainer(name string) (*PodmanSessionIntegration, int, string) {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

// RunNginxWithHealthCheck runs the alpine nginx container with an optional name and adds a healthcheck into it
func (p *PodmanTestIntegration) RunNginxWithHealthCheck(name string) (*PodmanSessionIntegration, string) {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-dt", "-P", "--health-cmd", "curl http://localhost/", nginx)
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.OutputToString()
}

func (p *PodmanTestIntegration) RunLsContainerInPod(name, pod string) (*PodmanSessionIntegration, int, string) {
	var podmanArgs = []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

// BuildImage uses podman build and buildah to build an image
// called imageName based on a string dockerfile
func (p *PodmanTestIntegration) BuildImage(dockerfile, imageName string, layers string) {
	dockerfilePath := filepath.Join(p.TempDir, "Dockerfile")
	err := ioutil.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
	Expect(err).To(BeNil())
	session := p.PodmanNoCache([]string{"build", "--layers=" + layers, "-t", imageName, "--file", dockerfilePath, p.TempDir})
	session.Wait(120)
	Expect(session.ExitCode()).To(Equal(0))
}

// PodmanPID execs podman and returns its PID
func (p *PodmanTestIntegration) PodmanPID(args []string) (*PodmanSessionIntegration, int) {
	podmanOptions := p.MakeOptions(args, false, false)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s", strings.Join(podmanOptions, " ")))
	}
	podmanSession := &PodmanSession{session}
	return &PodmanSessionIntegration{podmanSession}, command.Process.Pid
}

// Cleanup cleans up the temporary store
func (p *PodmanTestIntegration) Cleanup() {
	// Remove all containers
	stopall := p.Podman([]string{"stop", "-a", "--time", "0"})
	stopall.Wait(90)

	podstop := p.Podman([]string{"pod", "stop", "-a", "-t", "0"})
	podstop.WaitWithDefaultTimeout()
	podrm := p.Podman([]string{"pod", "rm", "-fa"})
	podrm.WaitWithDefaultTimeout()

	session := p.Podman([]string{"rm", "-fa"})
	session.Wait(90)

	p.StopRemoteService()
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()
}

// CleanupVolume cleans up the temporary store
func (p *PodmanTestIntegration) CleanupVolume() {
	// Remove all containers
	session := p.Podman([]string{"volume", "rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// PullImages pulls multiple images
func (p *PodmanTestIntegration) PullImages(images []string) error {
	for _, i := range images {
		p.PullImage(i)
	}
	return nil
}

// PullImage pulls a single image
// TODO should the timeout be configurable?
func (p *PodmanTestIntegration) PullImage(image string) error {
	session := p.PodmanNoCache([]string{"pull", image})
	session.Wait(60)
	Expect(session.ExitCode()).To(Equal(0))
	return nil
}

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSessionIntegration) InspectContainerToJSON() []define.InspectContainerData {
	var i []define.InspectContainerData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectPodToJSON takes the sessions output from a pod inspect and returns json
func (s *PodmanSessionIntegration) InspectPodToJSON() define.InspectPodData {
	var i define.InspectPodData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// CreatePod creates a pod with no infra container
// it optionally takes a pod name
func (p *PodmanTestIntegration) CreatePod(name string) (*PodmanSessionIntegration, int, string) {
	var podmanArgs = []string{"pod", "create", "--infra=false", "--share", ""}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

// CreatePod creates a pod with no infra container and some labels.
// it optionally takes a pod name
func (p *PodmanTestIntegration) CreatePodWithLabels(name string, labels map[string]string) (*PodmanSessionIntegration, int, string) {
	var podmanArgs = []string{"pod", "create", "--infra=false", "--share", ""}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	for labelKey, labelValue := range labels {
		podmanArgs = append(podmanArgs, "--label", fmt.Sprintf("%s=%s", labelKey, labelValue))
	}
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

func (p *PodmanTestIntegration) RunTopContainerInPod(name, pod string) *PodmanSessionIntegration {
	var podmanArgs = []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

func (p *PodmanTestIntegration) ImageExistsInMainStore(idOrName string) bool {
	results := p.PodmanNoCache([]string{"image", "exists", idOrName})
	results.WaitWithDefaultTimeout()
	return Expect(results.ExitCode()).To(Equal(0))
}

func (p *PodmanTestIntegration) RunHealthCheck(cid string) error {
	for i := 0; i < 10; i++ {
		hc := p.Podman([]string{"healthcheck", "run", cid})
		hc.WaitWithDefaultTimeout()
		if hc.ExitCode() == 0 {
			return nil
		}
		// Restart container if it's not running
		ps := p.Podman([]string{"ps", "--no-trunc", "--quiet", "--filter", fmt.Sprintf("id=%s", cid)})
		ps.WaitWithDefaultTimeout()
		if ps.ExitCode() == 0 {
			if !strings.Contains(ps.OutputToString(), cid) {
				fmt.Printf("Container %s is not running, restarting", cid)
				restart := p.Podman([]string{"restart", cid})
				restart.WaitWithDefaultTimeout()
				if restart.ExitCode() != 0 {
					return errors.Errorf("unable to restart %s", cid)
				}
			}
		}
		fmt.Printf("Waiting for %s to pass healthcheck\n", cid)
		time.Sleep(1 * time.Second)
	}
	return errors.Errorf("unable to detect %s as running", cid)
}

func (p *PodmanTestIntegration) CreateSeccompJson(in []byte) (string, error) {
	jsonFile := filepath.Join(p.TempDir, "seccomp.json")
	err := WriteJsonFile(in, jsonFile)
	if err != nil {
		return "", err
	}
	return jsonFile, nil
}
