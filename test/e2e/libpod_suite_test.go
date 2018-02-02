package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"encoding/json"
	"github.com/containers/image/copy"
	"github.com/containers/image/signature"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	sstorage "github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/inspect"
)

// - CRIO_ROOT=/var/tmp/checkout PODMAN_BINARY=/usr/bin/podman CONMON_BINARY=/usr/libexec/crio/conmon PAPR=1 sh .papr.sh
// PODMAN_OPTIONS="--root $TESTDIR/crio $STORAGE_OPTIONS --runroot $TESTDIR/crio-run --runtime ${RUNTIME_BINARY} --conmon ${CONMON_BINARY} --cni-config-dir ${LIBPOD_CNI_CONFIG}"

//TODO do the image caching
// "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=${IMAGES[${key}]} --import-from=dir:"$ARTIFACTS_PATH"/${key} --add-name=${IMAGES[${key}]}
//TODO whats the best way to clean up after a test

var (
	PODMAN_BINARY      string
	CONMON_BINARY      string
	CNI_CONFIG_DIR     string
	RUNC_BINARY        string
	INTEGRATION_ROOT   string
	STORAGE_OPTIONS    = "--storage-driver vfs"
	ARTIFACT_DIR       = "/tmp/.artifacts"
	IMAGES             = []string{"alpine", "busybox"}
	ALPINE             = "docker.io/library/alpine:latest"
	BB_GLIBC           = "docker.io/library/busybox:glibc"
	fedoraMinimal      = "registry.fedoraproject.org/fedora-minimal:latest"
	defaultWaitTimeout = 90
)

// PodmanSession wrapps the gexec.session so we can extend it
type PodmanSession struct {
	*gexec.Session
}

// PodmanTest struct for command line options
type PodmanTest struct {
	PodmanBinary        string
	ConmonBinary        string
	CrioRoot            string
	CNIConfigDir        string
	RunCBinary          string
	RunRoot             string
	StorageOptions      string
	SignaturePolicyPath string
	ArtifactPath        string
	TempDir             string
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	if reexec.Init() {
		os.Exit(1)
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var _ = BeforeSuite(func() {
	//Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := PodmanCreate("/tmp")
	podman.ArtifactPath = ARTIFACT_DIR
	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}
	for _, image := range IMAGES {
		fmt.Printf("Caching %s...\n", image)
		if err := podman.CreateArtifact(image); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}

})

// CreateTempDirin
func CreateTempDirInTempDir() (string, error) {
	return ioutil.TempDir("", "podman_test")
}

// PodmanCreate creates a PodmanTest instance for the tests
func PodmanCreate(tempDir string) PodmanTest {
	cwd, _ := os.Getwd()

	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	conmonBinary := filepath.Join("/usr/libexec/crio/conmon")
	if os.Getenv("CONMON_BINARY") != "" {
		conmonBinary = os.Getenv("CONMON_BINARY")
	}
	storageOptions := STORAGE_OPTIONS
	if os.Getenv("STORAGE_OPTIONS") != "" {
		storageOptions = os.Getenv("STORAGE_OPTIONS")
	}

	runCBinary := "/usr/bin/runc"
	CNIConfigDir := "/etc/cni/net.d"

	return PodmanTest{
		PodmanBinary:        podmanBinary,
		ConmonBinary:        conmonBinary,
		CrioRoot:            filepath.Join(tempDir, "crio"),
		CNIConfigDir:        CNIConfigDir,
		RunCBinary:          runCBinary,
		RunRoot:             filepath.Join(tempDir, "crio-run"),
		StorageOptions:      storageOptions,
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		ArtifactPath:        ARTIFACT_DIR,
		TempDir:             tempDir,
	}
}

//MakeOptions assembles all the podman main options
func (p *PodmanTest) MakeOptions() []string {
	return strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s",
		p.CrioRoot, p.RunRoot, p.RunCBinary, p.ConmonBinary, p.CNIConfigDir), " ")
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTest) Podman(args []string) *PodmanSession {
	podmanOptions := p.MakeOptions()
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s", strings.Join(podmanOptions, " ")))
	}
	return &PodmanSession{session}
}

// Cleanup cleans up the temporary store
func (p *PodmanTest) Cleanup() {
	// Remove all containers
	session := p.Podman([]string{"rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// GrepString takes session output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) GrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range strings.Split(s.OutputToString(), "\n") {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// Pull Images pulls multiple images
func (p *PodmanTest) PullImages(images []string) error {
	for _, i := range images {
		p.PullImage(i)
	}
	return nil
}

// Pull Image a single image
// TODO should the timeout be configurable?
func (p *PodmanTest) PullImage(image string) error {
	session := p.Podman([]string{"pull", image})
	session.Wait(60)
	Expect(session.ExitCode()).To(Equal(0))
	return nil
}

// OutputToString formats session output to string
func (s *PodmanSession) OutputToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Out.Contents()))
	return strings.Join(fields, " ")
}

// OutputToStringArray returns the output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) OutputToStringArray() []string {
	output := fmt.Sprintf("%s", s.Out.Contents())
	return strings.Split(output, "\n")
}

// IsJSONOutputValid attempts to unmarshall the session buffer
// and if successful, returns true, else false
func (s *PodmanSession) IsJSONOutputValid() bool {
	var i interface{}
	if err := json.Unmarshal(s.Out.Contents(), &i); err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSession) InspectContainerToJSON() inspect.ContainerData {
	var i inspect.ContainerData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSession) InspectImageJSON() inspect.ImageData {
	var i inspect.ImageData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

func (s *PodmanSession) WaitWithDefaultTimeout() {
	s.Wait(defaultWaitTimeout)
}

// SystemExec is used to exec a system command to check its exit code or output
func (p *PodmanTest) SystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	session, err := gexec.Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	return &PodmanSession{session}
}

// CreateArtifact creates a cached image in the artifact dir
func (p *PodmanTest) CreateArtifact(image string) error {
	imageName := fmt.Sprintf("docker://%s", image)
	systemContext := types.SystemContext{
		SignaturePolicyPath: p.SignaturePolicyPath,
	}
	policy, err := signature.DefaultPolicy(&systemContext)
	if err != nil {
		return errors.Errorf("error loading signature policy: %v", err)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return errors.Errorf("error loading signature policy: %v", err)
	}
	defer func() {
		_ = policyContext.Destroy()
	}()
	options := &copy.Options{}

	importRef, err := alltransports.ParseImageName(imageName)
	if err != nil {
		return errors.Errorf("error parsing image name %v: %v", image, err)
	}

	exportTo := filepath.Join("dir:", p.ArtifactPath, image)
	exportRef, err := alltransports.ParseImageName(exportTo)
	if err != nil {
		return errors.Errorf("error parsing image name %v: %v", exportTo, err)
	}

	return copy.Image(policyContext, exportRef, importRef, options)

	return nil
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTest) RestoreArtifact(image string) error {
	storeOptions := sstorage.DefaultStoreOptions
	storeOptions.GraphDriverName = "vfs"
	//storeOptions.GraphDriverOptions = storageOptions
	storeOptions.GraphRoot = p.CrioRoot
	storeOptions.RunRoot = p.RunRoot
	store, err := sstorage.GetStore(storeOptions)

	options := &copy.Options{}
	if err != nil {
		return errors.Errorf("error opening storage: %v", err)
	}
	defer func() {
		_, _ = store.Shutdown(false)
	}()

	storage.Transport.SetStore(store)
	ref, err := storage.Transport.ParseStoreReference(store, image)
	if err != nil {
		return errors.Errorf("error parsing image name: %v", err)
	}

	importFrom := fmt.Sprintf("dir:%s", filepath.Join(p.ArtifactPath, image))
	importRef, err := alltransports.ParseImageName(importFrom)
	if err != nil {
		return errors.Errorf("error parsing image name %v: %v", image, err)
	}
	systemContext := types.SystemContext{
		SignaturePolicyPath: p.SignaturePolicyPath,
	}
	policy, err := signature.DefaultPolicy(&systemContext)
	if err != nil {
		return errors.Errorf("error loading signature policy: %v", err)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return errors.Errorf("error loading signature policy: %v", err)
	}
	defer func() {
		_ = policyContext.Destroy()
	}()
	err = copy.Image(policyContext, ref, importRef, options)
	if err != nil {
		return errors.Errorf("error importing %s: %v", importFrom, err)
	}
	return nil
}

// RestoreAllArtifacts unpacks all cached images
func (p *PodmanTest) RestoreAllArtifacts() error {
	for _, image := range IMAGES {
		if err := p.RestoreArtifact(image); err != nil {
			return err
		}
	}
	return nil
}

//RunSleepContainer runs a simple container in the background that
// sleeps.  If the name passed != "", it will have a name
func (p *PodmanTest) RunSleepContainer(name string) *PodmanSession {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "sleep", "90")
	return p.Podman(podmanArgs)
}

//RunLsContainer runs a simple container in the background that
// simply runs ls. If the name passed != "", it will have a name
func (p *PodmanTest) RunLsContainer(name string) (*PodmanSession, int, string) {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

//NumberOfContainersRunning returns an int of how many
// containers are currently running.
func (p *PodmanTest) NumberOfContainersRunning() int {
	var containers []string
	ps := p.Podman([]string{"ps", "-q"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

//NumberOfContainersreturns an int of how many
// containers are currently defined.
func (p *PodmanTest) NumberOfContainers() int {
	var containers []string
	ps := p.Podman([]string{"ps", "-aq"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// StringInSlice determines if a string is in a string slice, returns bool
func StringInSlice(s string, sl []string) bool {
	for _, i := range sl {
		if i == s {
			return true
		}
	}
	return false
}
