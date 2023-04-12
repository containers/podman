package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	crypto_rand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/sirupsen/logrus"

	"github.com/containers/storage/pkg/parsers/kernel"
	. "github.com/onsi/ginkgo/v2"    //nolint:revive,stylecheck
	. "github.com/onsi/gomega"       //nolint:revive,stylecheck
	. "github.com/onsi/gomega/gexec" //nolint:revive,stylecheck
)

type NetworkBackend int

const (
	// Container Networking backend
	CNI NetworkBackend = iota
	// Netavark network backend
	Netavark NetworkBackend = iota
	// Env variable for creating time files.
	EnvTimeDir = "_PODMAN_TIME_DIR"
)

func (n NetworkBackend) ToString() string {
	switch n {
	case CNI:
		return "cni"
	case Netavark:
		return "netavark"
	}
	logrus.Errorf("unknown network backend: %q", n)
	return ""
}

var (
	DefaultWaitTimeout   = 90
	OSReleasePath        = "/etc/os-release"
	ProcessOneCgroupPath = "/proc/1/cgroup"
)

// PodmanTestCommon contains common functions will be updated later in
// the inheritance structs
type PodmanTestCommon interface {
	MakeOptions(args []string, noEvents, noCache bool) []string
	WaitForContainer() bool
	WaitContainerReady(id string, expStr string, timeout int, step int) bool
}

// PodmanTest struct for command line options
type PodmanTest struct {
	ImageCacheDir      string
	ImageCacheFS       string
	NetworkBackend     NetworkBackend
	DatabaseBackend    string
	PodmanBinary       string
	PodmanMakeOptions  func(args []string, noEvents, noCache bool) []string
	RemoteCommand      *exec.Cmd
	RemotePodmanBinary string
	RemoteSession      *os.Process
	RemoteSocket       string
	RemoteSocketLock   string // If not "", should be removed _after_ RemoteSocket is removed
	RemoteTest         bool
	TempDir            string
}

// PodmanSession wraps the gexec.session so we can extend it
type PodmanSession struct {
	*Session
}

// HostOS is a simple struct for the test os
type HostOS struct {
	Distribution string
	Version      string
	Arch         string
}

// MakeOptions assembles all podman options
func (p *PodmanTest) MakeOptions(args []string, noEvents, noCache bool) []string {
	return p.PodmanMakeOptions(args, noEvents, noCache)
}

// PodmanAsUserBase exec podman as user. uid and gid is set for credentials usage. env is used
// to record the env for debugging
func (p *PodmanTest) PodmanAsUserBase(args []string, uid, gid uint32, cwd string, env []string, noEvents, noCache bool, wrapper []string, extraFiles []*os.File) *PodmanSession {
	var command *exec.Cmd
	podmanOptions := p.MakeOptions(args, noEvents, noCache)
	podmanBinary := p.PodmanBinary
	if p.RemoteTest {
		podmanBinary = p.RemotePodmanBinary
	}

	if timeDir := os.Getenv(EnvTimeDir); timeDir != "" {
		timeFile, err := os.CreateTemp(timeDir, ".time")
		if err != nil {
			Fail(fmt.Sprintf("Error creating time file: %v", err))
		}
		timeArgs := []string{"-f", "%M", "-o", timeFile.Name()}
		timeCmd := append([]string{"/usr/bin/time"}, timeArgs...)
		wrapper = append(timeCmd, wrapper...)
	}
	runCmd := wrapper
	runCmd = append(runCmd, podmanBinary)

	if env == nil {
		fmt.Printf("Running: %s %s\n", strings.Join(runCmd, " "), strings.Join(podmanOptions, " "))
	} else {
		fmt.Printf("Running: (env: %v) %s %s\n", env, strings.Join(runCmd, " "), strings.Join(podmanOptions, " "))
	}
	if uid != 0 || gid != 0 {
		pythonCmd := fmt.Sprintf("import os; import sys; uid = %d; gid = %d; cwd = '%s'; os.setgid(gid); os.setuid(uid); os.chdir(cwd) if len(cwd)>0 else True; os.execv(sys.argv[1], sys.argv[1:])", gid, uid, cwd)
		runCmd = append(runCmd, podmanOptions...)
		nsEnterOpts := append([]string{"-c", pythonCmd}, runCmd...)
		command = exec.Command("python", nsEnterOpts...)
	} else {
		runCmd = append(runCmd, podmanOptions...)
		command = exec.Command(runCmd[0], runCmd[1:]...)
	}
	if env != nil {
		command.Env = env
	}
	if cwd != "" {
		command.Dir = cwd
	}

	command.ExtraFiles = extraFiles

	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s\n%v", strings.Join(podmanOptions, " "), err))
	}
	return &PodmanSession{session}
}

// PodmanBase exec podman with default env.
func (p *PodmanTest) PodmanBase(args []string, noEvents, noCache bool) *PodmanSession {
	return p.PodmanAsUserBase(args, 0, 0, "", nil, noEvents, noCache, nil, nil)
}

// WaitForContainer waits on a started container
func (p *PodmanTest) WaitForContainer() bool {
	for i := 0; i < 10; i++ {
		if p.NumberOfContainersRunning() > 0 {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("WaitForContainer(): timed out\n")
	return false
}

// NumberOfContainersRunning returns an int of how many
// containers are currently running.
func (p *PodmanTest) NumberOfContainersRunning() int {
	var containers []string
	ps := p.PodmanBase([]string{"ps", "-q"}, false, true)
	ps.WaitWithDefaultTimeout()
	Expect(ps).Should(Exit(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfContainers returns an int of how many
// containers are currently defined.
func (p *PodmanTest) NumberOfContainers() int {
	var containers []string
	ps := p.PodmanBase([]string{"ps", "-aq"}, false, true)
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfPods returns an int of how many
// pods are currently defined.
func (p *PodmanTest) NumberOfPods() int {
	var pods []string
	ps := p.PodmanBase([]string{"pod", "ps", "-q"}, false, true)
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			pods = append(pods, i)
		}
	}
	return len(pods)
}

// GetContainerStatus returns the containers state.
// This function assumes only one container is active.
func (p *PodmanTest) GetContainerStatus() string {
	var podmanArgs = []string{"ps"}
	podmanArgs = append(podmanArgs, "--all", "--format={{.Status}}")
	session := p.PodmanBase(podmanArgs, false, true)
	session.WaitWithDefaultTimeout()
	return session.OutputToString()
}

// WaitContainerReady waits process or service inside container start, and ready to be used.
func (p *PodmanTest) WaitContainerReady(id string, expStr string, timeout int, step int) bool {
	startTime := time.Now()
	s := p.PodmanBase([]string{"logs", id}, false, true)
	s.WaitWithDefaultTimeout()

	for {
		if time.Since(startTime) >= time.Duration(timeout)*time.Second {
			fmt.Printf("Container %s is not ready in %ds", id, timeout)
			return false
		}

		if strings.Contains(s.OutputToString(), expStr) || strings.Contains(s.ErrorToString(), expStr) {
			return true
		}
		time.Sleep(time.Duration(step) * time.Second)
		s = p.PodmanBase([]string{"logs", id}, false, true)
		s.WaitWithDefaultTimeout()
	}
}

// WaitForContainer is a wrapper function for accept inheritance PodmanTest struct.
func WaitForContainer(p PodmanTestCommon) bool {
	return p.WaitForContainer()
}

// WaitForContainerReady is a wrapper function for accept inheritance PodmanTest struct.
func WaitContainerReady(p PodmanTestCommon, id string, expStr string, timeout int, step int) bool {
	return p.WaitContainerReady(id, expStr, timeout, step)
}

// OutputToString formats session output to string
func (s *PodmanSession) OutputToString() string {
	if s == nil || s.Out == nil || s.Out.Contents() == nil {
		return ""
	}

	fields := strings.Fields(string(s.Out.Contents()))
	return strings.Join(fields, " ")
}

// OutputToStringArray returns the output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) OutputToStringArray() []string {
	var results []string
	output := string(s.Out.Contents())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// ErrorToString formats session stderr to string
func (s *PodmanSession) ErrorToString() string {
	fields := strings.Fields(string(s.Err.Contents()))
	return strings.Join(fields, " ")
}

// ErrorToStringArray returns the stderr output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) ErrorToStringArray() []string {
	output := string(s.Err.Contents())
	return strings.Split(output, "\n")
}

// GrepString takes session output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) GrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range s.OutputToStringArray() {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// ErrorGrepString takes session stderr output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) ErrorGrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range s.ErrorToStringArray() {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// LineInOutputStartsWith returns true if a line in a
// session output starts with the supplied string
func (s *PodmanSession) LineInOutputStartsWith(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.HasPrefix(i, term) {
			return true
		}
	}
	return false
}

// LineInOutputContains returns true if a line in a
// session output contains the supplied string
func (s *PodmanSession) LineInOutputContains(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.Contains(i, term) {
			return true
		}
	}
	return false
}

// LineInOutputContainsTag returns true if a line in the
// session's output contains the repo-tag pair as returned
// by podman-images(1).
func (s *PodmanSession) LineInOutputContainsTag(repo, tag string) bool {
	tagMap := tagOutputToMap(s.OutputToStringArray())
	return tagMap[repo][tag]
}

// IsJSONOutputValid attempts to unmarshal the session buffer
// and if successful, returns true, else false
func (s *PodmanSession) IsJSONOutputValid() bool {
	var i interface{}
	if err := json.Unmarshal(s.Out.Contents(), &i); err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

// WaitWithDefaultTimeout waits for process finished with DefaultWaitTimeout
func (s *PodmanSession) WaitWithDefaultTimeout() {
	s.WaitWithTimeout(DefaultWaitTimeout)
}

// WaitWithTimeout waits for process finished with DefaultWaitTimeout
func (s *PodmanSession) WaitWithTimeout(timeout int) {
	Eventually(s, timeout).Should(Exit(), func() string {
		// in case of timeouts show output
		return fmt.Sprintf("command timed out after %ds: %v\nSTDOUT: %s\nSTDERR: %s",
			timeout, s.Command.Args, string(s.Out.Contents()), string(s.Err.Contents()))
	})
	os.Stdout.Sync()
	os.Stderr.Sync()
	fmt.Println("output:", s.OutputToString())
}

// CreateTempDirInTempDir create a temp dir with prefix podman_test
func CreateTempDirInTempDir() (string, error) {
	return os.MkdirTemp("", "podman_test")
}

// SystemExec is used to exec a system command to check its exit code or output
func SystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	fmt.Println("Execing " + c.String() + "\n")
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	session.Wait(DefaultWaitTimeout)
	return &PodmanSession{session}
}

// StartSystemExec is used to start exec a system command
func StartSystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	fmt.Println("Execing " + c.String() + "\n")
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	return &PodmanSession{session}
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

// tagOutPutToMap parses each string in imagesOutput and returns
// a map whose key is a repo, and value is another map whose keys
// are the tags found for that repo. Notice, the first array item will
// be skipped as it's considered to be the header.
func tagOutputToMap(imagesOutput []string) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	// iterate over output but skip the header
	for _, i := range imagesOutput[1:] {
		tmp := []string{}
		for _, x := range strings.Split(i, " ") {
			if x != "" {
				tmp = append(tmp, x)
			}
		}
		// podman-images(1) return a list like output
		// in the format of "Repository Tag [...]"
		if len(tmp) < 2 {
			continue
		}
		if m[tmp[0]] == nil {
			m[tmp[0]] = map[string]bool{}
		}
		m[tmp[0]][tmp[1]] = true
	}
	return m
}

// GetHostDistributionInfo returns a struct with its distribution Name and version
func GetHostDistributionInfo() HostOS {
	f, err := os.Open(OSReleasePath)
	if err != nil {
		return HostOS{}
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	host := HostOS{}
	host.Arch = runtime.GOARCH
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			host.Distribution = strings.ReplaceAll(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "")
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			host.Version = strings.ReplaceAll(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "")
		}
	}
	return host
}

// IsKernelNewerThan compares the current kernel version to one provided.  If
// the kernel is equal to or greater, returns true
func IsKernelNewerThan(version string) (bool, error) {
	inputVersion, err := kernel.ParseRelease(version)
	if err != nil {
		return false, err
	}
	kv, err := kernel.GetKernelVersion()
	if err != nil {
		return false, err
	}

	// CompareKernelVersion compares two kernel.VersionInfo structs.
	// Returns -1 if a < b, 0 if a == b, 1 it a > b
	result := kernel.CompareKernelVersion(*kv, *inputVersion)
	if result >= 0 {
		return true, nil
	}
	return false, nil
}

// IsCommandAvailable check if command exist
func IsCommandAvailable(command string) bool {
	check := exec.Command("bash", "-c", strings.Join([]string{"command -v", command}, " "))
	err := check.Run()
	return err == nil
}

// WriteJSONFile write json format data to a json file
func WriteJSONFile(data []byte, filePath string) error {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return err
	}
	formatJSON, err := json.MarshalIndent(jsonData, "", "	")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, formatJSON, 0644)
}

// Containerized check the podman command run inside container
func Containerized() bool {
	container := os.Getenv("container")
	if container != "" {
		return true
	}
	b, err := os.ReadFile(ProcessOneCgroupPath)
	if err != nil {
		// shrug, if we cannot read that file, return false
		return false
	}
	return strings.Contains(string(b), "docker")
}

var randomLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString returns a string of given length composed of random characters
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = randomLetters[rand.Intn(len(randomLetters))]
	}
	return string(b)
}

// Encode *rsa.PublicKey and store it in a file.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the file storing the public key.
func savePublicKey(fileName string, publicKey *rsa.PublicKey) (string, error) {
	// Encode public key to PKIX, ASN.1 DER form
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubBytes,
		},
	)

	// Write public key to file
	publicKeyFileName := fileName + ".rsa.pub"
	if err := os.WriteFile(publicKeyFileName, pubPEM, 0600); err != nil {
		return "", err
	}

	return publicKeyFileName, nil
}

// Encode *rsa.PrivateKey and store it in a file.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the file storing the private key.
func savePrivateKey(fileName string, privateKey *rsa.PrivateKey) (string, error) {
	// Encode private key to PKCS#1, ASN.1 DER form
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privBytes,
		},
	)

	// Write private key to file
	privateKeyFileName := fileName + ".rsa"
	if err := os.WriteFile(privateKeyFileName, keyPEM, 0600); err != nil {
		return "", err
	}

	return privateKeyFileName, nil
}

// Generate RSA key pair of specified bit size and write them to files.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the files storing the public and private key respectively.
func WriteRSAKeyPair(fileName string, bitSize int) (string, string, error) {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(crypto_rand.Reader, bitSize)
	if err != nil {
		return "", "", err
	}

	publicKey := privateKey.Public().(*rsa.PublicKey)

	publicKeyFileName, err := savePublicKey(fileName, publicKey)
	if err != nil {
		return "", "", err
	}

	privateKeyFileName, err := savePrivateKey(fileName, privateKey)
	if err != nil {
		return "", "", err
	}

	return publicKeyFileName, privateKeyFileName, nil
}
