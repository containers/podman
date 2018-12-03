package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
	. "github.com/containers/libpod/test/utils"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
	RunCBinary          string
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
func PodmanTestCreate(tempDir string) *PodmanTestIntegration {

	host := GetHostDistributionInfo()
	cwd, _ := os.Getwd()

	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	conmonBinary := filepath.Join("/usr/libexec/podman/conmon")
	altConmonBinary := "/usr/libexec/crio/conmon"
	if _, err := os.Stat(altConmonBinary); err == nil {
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

	runCBinary, err := exec.LookPath("runc")
	// If we cannot find the runc binary, setting to something static as we have no way
	// to return an error.  The tests will fail and point out that the runc binary could
	// not be found nicely.
	if err != nil {
		runCBinary = "/usr/bin/runc"
	}

	CNIConfigDir := "/etc/cni/net.d"

	p := &PodmanTestIntegration{
		PodmanTest: PodmanTest{
			PodmanBinary: podmanBinary,
			ArtifactPath: ARTIFACT_DIR,
			TempDir:      tempDir,
		},
		ConmonBinary:        conmonBinary,
		CrioRoot:            filepath.Join(tempDir, "crio"),
		CNIConfigDir:        CNIConfigDir,
		RunCBinary:          runCBinary,
		RunRoot:             filepath.Join(tempDir, "crio-run"),
		StorageOptions:      storageOptions,
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		CgroupManager:       cgroupManager,
		Host:                host,
	}

	// Setup registries.conf ENV variable
	p.setDefaultRegistriesConfigEnv()
	// Rewrite the PodmanAsUser function
	p.PodmanMakeOptions = p.makeOptions
	return p
}

//MakeOptions assembles all the podman main options
func (p *PodmanTestIntegration) makeOptions(args []string) []string {
	podmanOptions := strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s --cgroup-manager %s",
		p.CrioRoot, p.RunRoot, p.RunCBinary, p.ConmonBinary, p.CNIConfigDir, p.CgroupManager), " ")
	if os.Getenv("HOOK_OPTION") != "" {
		podmanOptions = append(podmanOptions, os.Getenv("HOOK_OPTION"))
	}
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTestIntegration) Podman(args []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanBase(args)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanAsUser is the exec call to podman on the filesystem with the specified uid/gid and environment
func (p *PodmanTestIntegration) PodmanAsUser(args []string, uid, gid uint32, env []string) *PodmanSessionIntegration {
	podmanSession := p.PodmanAsUserBase(args, uid, gid, env)
	return &PodmanSessionIntegration{podmanSession}
}

// PodmanPID execs podman and returns its PID
func (p *PodmanTestIntegration) PodmanPID(args []string) (*PodmanSessionIntegration, int) {
	podmanOptions := p.MakeOptions(args)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s", strings.Join(podmanOptions, " ")))
	}
	podmanSession := &PodmanSession{session}
	return &PodmanSessionIntegration{podmanSession}, command.Process.Pid
}

// Cleanup cleans up the temporary store
func (p *PodmanTestIntegration) Cleanup() {
	// Remove all containers
	stopall := p.Podman([]string{"stop", "-a", "--timeout", "0"})
	stopall.WaitWithDefaultTimeout()
	session := p.Podman([]string{"rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()
}

// CleanupPod cleans up the temporary store
func (p *PodmanTestIntegration) CleanupPod() {
	// Remove all containers
	session := p.Podman([]string{"pod", "rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
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
	session := p.Podman([]string{"pull", image})
	session.Wait(60)
	Expect(session.ExitCode()).To(Equal(0))
	return nil
}

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSessionIntegration) InspectContainerToJSON() []inspect.ContainerData {
	var i []inspect.ContainerData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectPodToJSON takes the sessions output from a pod inspect and returns json
func (s *PodmanSessionIntegration) InspectPodToJSON() libpod.PodInspect {
	var i libpod.PodInspect
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSessionIntegration) InspectImageJSON() []inspect.ImageData {
	var i []inspect.ImageData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
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

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTestIntegration) RestoreArtifact(image string) error {
	fmt.Printf("Restoring %s...\n", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	restore := p.Podman([]string{"load", "-q", "-i", destName})
	restore.Wait(90)
	return nil
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

//RunTopContainer runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name
func (p *PodmanTestIntegration) RunTopContainer(name string) *PodmanSessionIntegration {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

func (p *PodmanTestIntegration) RunTopContainerInPod(name, pod string) *PodmanSessionIntegration {
	var podmanArgs = []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

//RunLsContainer runs a simple container in the background that
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
	session := p.Podman([]string{"build", "--layers=" + layers, "-t", imageName, "--file", dockerfilePath, p.TempDir})
	session.Wait(120)
	Expect(session.ExitCode()).To(Equal(0))
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
