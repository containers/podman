package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/cgroups"
	"github.com/containers/podman/v3/pkg/inspect"
	"github.com/containers/podman/v3/pkg/rootless"
	. "github.com/containers/podman/v3/test/utils"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/stringid"
	jsoniter "github.com/json-iterator/go"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	PODMAN_BINARY      string
	CONMON_BINARY      string
	CNI_CONFIG_DIR     string
	RUNC_BINARY        string
	INTEGRATION_ROOT   string
	CGROUP_MANAGER     = "systemd"
	ARTIFACT_DIR       = "/tmp/.artifacts"
	RESTORE_IMAGES     = []string{ALPINE, BB, nginx}
	defaultWaitTimeout = 90
	CGROUPSV2, _       = cgroups.IsCgroup2UnifiedMode()
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

// PodmanSessionIntegration struct for command line session
type PodmanSessionIntegration struct {
	*PodmanSession
}

type testResult struct {
	name   string
	length float64
}

var noCache = "Cannot run nocache with remote"

type testResultsSorted []testResult

func (a testResultsSorted) Len() int      { return len(a) }
func (a testResultsSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type testResultsSortedLength struct{ testResultsSorted }

func (a testResultsSorted) Less(i, j int) bool { return a[i].length < a[j].length }

var testResults []testResult
var testResultsMutex sync.Mutex

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
	// make cache dir
	if err := os.MkdirAll(ImageCacheDir, 0777); err != nil {
		fmt.Printf("%q\n", err)
		os.Exit(1)
	}

	// Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := PodmanTestSetup("/tmp")
	podman.ArtifactPath = ARTIFACT_DIR
	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}

	// Pull cirros but don't put it into the cache
	pullImages := []string{cirros, fedoraToolbox, volumeTest}
	pullImages = append(pullImages, CACHE_IMAGES...)
	for _, image := range pullImages {
		podman.createArtifact(image)
	}

	if err := os.MkdirAll(filepath.Join(ImageCacheDir, podman.ImageCacheFS+"-images"), 0777); err != nil {
		fmt.Printf("%q\n", err)
		os.Exit(1)
	}
	podman.CrioRoot = ImageCacheDir
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

	// If running remote, we need to stop the associated podman system service
	if podman.RemoteTest {
		podman.StopRemoteService()
	}

	return []byte(path)
}, func(data []byte) {
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
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

		// If running remote, we need to stop the associated podman system service
		if podmanTest.RemoteTest {
			podmanTest.StopRemoteService()
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
		ociRuntime = "crun"
	}
	os.Setenv("DISABLE_HC_SYSTEMD", "true")
	CNIConfigDir := "/etc/cni/net.d"
	if rootless.IsRootless() {
		CNIConfigDir = filepath.Join(os.Getenv("HOME"), ".config/cni/net.d")
	}
	if err := os.MkdirAll(CNIConfigDir, 0755); err != nil {
		panic(err)
	}

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

func (p PodmanTestIntegration) AddImageToRWStore(image string) {
	if err := p.RestoreArtifact(image); err != nil {
		logrus.Errorf("unable to restore %s to RW store", image)
	}
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
		pull.Wait(440)
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
	testResultsMutex.Lock()
	testResults = append(testResults, tr)
	testResultsMutex.Unlock()
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
	return p.RunTopContainerWithArgs(name, nil)
}

// RunTopContainerWithArgs runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name, command args can also be passed in
func (p *PodmanTestIntegration) RunTopContainerWithArgs(name string, args []string) *PodmanSessionIntegration {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, args...)
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
	if session.ExitCode() != 0 {
		return session, session.ExitCode(), session.OutputToString()
	}
	cid := session.OutputToString()

	wsession := p.Podman([]string{"wait", cid})
	wsession.WaitWithDefaultTimeout()
	return session, wsession.ExitCode(), cid
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
	if session.ExitCode() != 0 {
		return session, session.ExitCode(), session.OutputToString()
	}
	cid := session.OutputToString()

	wsession := p.Podman([]string{"wait", cid})
	wsession.WaitWithDefaultTimeout()
	return session, wsession.ExitCode(), cid
}

// BuildImage uses podman build and buildah to build an image
// called imageName based on a string dockerfile
func (p *PodmanTestIntegration) BuildImage(dockerfile, imageName string, layers string) string {
	return p.buildImage(dockerfile, imageName, layers, "")
}

// BuildImageWithLabel uses podman build and buildah to build an image
// called imageName based on a string dockerfile, adds desired label to paramset
func (p *PodmanTestIntegration) BuildImageWithLabel(dockerfile, imageName string, layers string, label string) string {
	return p.buildImage(dockerfile, imageName, layers, label)
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
	stopall.WaitWithDefaultTimeout()

	podstop := p.Podman([]string{"pod", "stop", "-a", "-t", "0"})
	podstop.WaitWithDefaultTimeout()
	podrm := p.Podman([]string{"pod", "rm", "-fa"})
	podrm.WaitWithDefaultTimeout()

	session := p.Podman([]string{"rm", "-fa"})
	session.WaitWithDefaultTimeout()

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

	p.Cleanup()
}

// CleanupSecret cleans up the temporary store
func (p *PodmanTestIntegration) CleanupSecrets() {
	// Remove all containers
	session := p.Podman([]string{"secret", "rm", "-a"})
	session.Wait(90)

	// Stop remove service on secret cleanup
	p.StopRemoteService()

	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
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

// InspectPodToJSON takes the sessions output from an inspect and returns json
func (s *PodmanSessionIntegration) InspectPodArrToJSON() []define.InspectPodData {
	var i []define.InspectPodData
	err := jsoniter.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// CreatePod creates a pod with no infra container
// it optionally takes a pod name
func (p *PodmanTestIntegration) CreatePod(options map[string][]string) (*PodmanSessionIntegration, int, string) {
	var args = []string{"pod", "create", "--infra=false", "--share", ""}
	for k, values := range options {
		for _, v := range values {
			args = append(args, k+"="+v)
		}
	}

	session := p.Podman(args)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

func (p *PodmanTestIntegration) RunTopContainerInPod(name, pod string) *PodmanSessionIntegration {
	return p.RunTopContainerWithArgs(name, []string{"--pod", pod})
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

func checkReason(reason string) {
	if len(reason) < 5 {
		panic("Test must specify a reason to skip")
	}
}

func SkipIfRootlessCgroupsV1(reason string) {
	checkReason(reason)
	if os.Geteuid() != 0 && !CGROUPSV2 {
		Skip("[rootless]: " + reason)
	}
}

func SkipIfRootless(reason string) {
	checkReason(reason)
	if os.Geteuid() != 0 {
		ginkgo.Skip("[rootless]: " + reason)
	}
}

func SkipIfNotRootless(reason string) {
	checkReason(reason)
	if os.Geteuid() == 0 {
		ginkgo.Skip("[notRootless]: " + reason)
	}
}

func SkipIfNotSystemd(manager, reason string) {
	checkReason(reason)
	if manager != "systemd" {
		ginkgo.Skip("[notSystemd]: " + reason)
	}
}

func SkipIfNotFedora() {
	info := GetHostDistributionInfo()
	if info.Distribution != "fedora" {
		ginkgo.Skip("Test can only run on Fedora")
	}
}

func isRootless() bool {
	return os.Geteuid() != 0
}

func SkipIfCgroupV1(reason string) {
	checkReason(reason)
	if !CGROUPSV2 {
		Skip(reason)
	}
}

func SkipIfCgroupV2(reason string) {
	checkReason(reason)
	if CGROUPSV2 {
		Skip(reason)
	}
}

func isContainerized() bool {
	// This is set to "podman" by podman automatically
	if os.Getenv("container") != "" {
		return true
	}
	return false
}

func SkipIfContainerized(reason string) {
	checkReason(reason)
	if isContainerized() {
		Skip(reason)
	}
}

// PodmanAsUser is the exec call to podman on the filesystem with the specified uid/gid and environment
func (p *PodmanTestIntegration) PodmanAsUser(args []string, uid, gid uint32, cwd string, env []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanAsUserBase(args, uid, gid, cwd, env, false, false, nil)
	return &PodmanSessionIntegration{podmanSession}
}

// RestartRemoteService stop and start API Server, usually to change config
func (p *PodmanTestIntegration) RestartRemoteService() {
	p.StopRemoteService()
	p.StartRemoteService()
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

func populateCache(podman *PodmanTestIntegration) {
	for _, image := range CACHE_IMAGES {
		podman.RestoreArtifactToCache(image)
	}
	// logformatter uses this to recognize the first test
	fmt.Printf("-----------------------------\n")
}

func removeCache() {
	// Remove cache dirs
	if err := os.RemoveAll(ImageCacheDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// PodmanNoCache calls the podman command with no configured imagecache
func (p *PodmanTestIntegration) PodmanNoCache(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, false, true)
	return &PodmanSessionIntegration{podmanSession}
}

func PodmanTestSetup(tempDir string) *PodmanTestIntegration {
	return PodmanTestCreateUtil(tempDir, false)
}

// PodmanNoEvents calls the Podman command without an imagecache and without an
// events backend. It is used mostly for caching and uncaching images.
func (p *PodmanTestIntegration) PodmanNoEvents(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args, true, true)
	return &PodmanSessionIntegration{podmanSession}
}

// MakeOptions assembles all the podman main options
func (p *PodmanTestIntegration) makeOptions(args []string, noEvents, noCache bool) []string {
	if p.RemoteTest {
		return args
	}
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
	if !noCache {
		cacheOptions := []string{"--storage-opt",
			fmt.Sprintf("%s.imagestore=%s", p.PodmanTest.ImageCacheFS, p.PodmanTest.ImageCacheDir)}
		podmanOptions = append(cacheOptions, podmanOptions...)
	}
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

func writeConf(conf []byte, confPath string) {
	if err := ioutil.WriteFile(confPath, conf, 777); err != nil {
		fmt.Println(err)
	}
}

func removeConf(confPath string) {
	if err := os.Remove(confPath); err != nil {
		fmt.Println(err)
	}
}

// generateNetworkConfig generates a cni config with a random name
// it returns the network name and the filepath
func generateNetworkConfig(p *PodmanTestIntegration) (string, string) {
	// generate a random name to prevent conflicts with other tests
	name := "net" + stringid.GenerateNonCryptoID()
	path := filepath.Join(p.CNIConfigDir, fmt.Sprintf("%s.conflist", name))
	conf := fmt.Sprintf(`{
		"cniVersion": "0.3.0",
		"name": "%s",
		"plugins": [
		  {
			"type": "bridge",
			"bridge": "cni1",
			"isGateway": true,
			"ipMasq": true,
			"ipam": {
				"type": "host-local",
				"subnet": "10.99.0.0/16",
				"routes": [
					{ "dst": "0.0.0.0/0" }
				]
			}
		  },
		  {
			"type": "portmap",
			"capabilities": {
			  "portMappings": true
			}
		  }
		]
	}`, name)
	writeConf([]byte(conf), path)

	return name, path
}

func (p *PodmanTestIntegration) removeCNINetwork(name string) {
	session := p.Podman([]string{"network", "rm", "-f", name})
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(BeNumerically("<=", 1))
}

func (p *PodmanSessionIntegration) jq(jqCommand string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("jq", jqCommand)
	cmd.Stdin = strings.NewReader(p.OutputToString())
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"), err
}

func (p *PodmanTestIntegration) buildImage(dockerfile, imageName string, layers string, label string) string {
	dockerfilePath := filepath.Join(p.TempDir, "Dockerfile")
	err := ioutil.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
	Expect(err).To(BeNil())
	cmd := []string{"build", "--pull-never", "--layers=" + layers, "--file", dockerfilePath}
	if label != "" {
		cmd = append(cmd, "--label="+label)
	}
	if len(imageName) > 0 {
		cmd = append(cmd, []string{"-t", imageName}...)
	}
	cmd = append(cmd, p.TempDir)
	session := p.Podman(cmd)
	session.Wait(240)
	Expect(session).Should(Exit(0), fmt.Sprintf("BuildImage session output: %q", session.OutputToString()))
	output := session.OutputToStringArray()
	return output[len(output)-1]
}
