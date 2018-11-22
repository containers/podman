package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/containers/storage/pkg/parsers/kernel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	defaultWaitTimeout   = 90
	OSReleasePath        = "/etc/os-release"
	ProcessOneCgroupPath = "/proc/1/cgroup"
)

// PodmanTestCommon contains common functions will be updated later in
// the inheritance structs
type PodmanTestCommon interface {
	MakeOptions(args []string) []string
	WaitForContainer() bool
	WaitContainerReady(id string, expStr string, timeout int, step int) bool
}

// PodmanTest struct for command line options
type PodmanTest struct {
	PodmanMakeOptions func(args []string) []string
	PodmanBinary      string
	ArtifactPath      string
	TempDir           string
}

// PodmanSession wraps the gexec.session so we can extend it
type PodmanSession struct {
	*gexec.Session
}

// HostOS is a simple struct for the test os
type HostOS struct {
	Distribution string
	Version      string
	Arch         string
}

// MakeOptions assembles all podman options
func (p *PodmanTest) MakeOptions(args []string) []string {
	return p.PodmanMakeOptions(args)
}

// PodmanAsUserBase exec podman as user. uid and gid is set for credentials useage. env is used
// to record the env for debugging
func (p *PodmanTest) PodmanAsUserBase(args []string, uid, gid uint32, env []string) *PodmanSession {
	var command *exec.Cmd
	podmanOptions := p.MakeOptions(args)

	if env == nil {
		fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	} else {
		fmt.Printf("Running: (env: %v) %s %s\n", env, p.PodmanBinary, strings.Join(podmanOptions, " "))
	}
	if uid != 0 || gid != 0 {
		nsEnterOpts := append([]string{"--userspec", fmt.Sprintf("%d:%d", uid, gid), "/", p.PodmanBinary}, podmanOptions...)
		command = exec.Command("chroot", nsEnterOpts...)
	} else {
		command = exec.Command(p.PodmanBinary, podmanOptions...)
	}
	if env != nil {
		command.Env = env
	}

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s\n%v", strings.Join(podmanOptions, " "), err))
	}
	return &PodmanSession{session}
}

// PodmanBase exec podman with default env.
func (p *PodmanTest) PodmanBase(args []string) *PodmanSession {
	return p.PodmanAsUserBase(args, 0, 0, nil)
}

// WaitForContainer waits on a started container
func (p *PodmanTest) WaitForContainer() bool {
	for i := 0; i < 10; i++ {
		if p.NumberOfContainersRunning() > 0 {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// NumberOfContainersRunning returns an int of how many
// containers are currently running.
func (p *PodmanTest) NumberOfContainersRunning() int {
	var containers []string
	ps := p.PodmanBase([]string{"ps", "-q"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
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
	ps := p.PodmanBase([]string{"ps", "-aq"})
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
	ps := p.PodmanBase([]string{"pod", "ps", "-q"})
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
	session := p.PodmanBase(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session.OutputToString()
}

// WaitContainerReady waits process or service inside container start, and ready to be used.
func (p *PodmanTest) WaitContainerReady(id string, expStr string, timeout int, step int) bool {
	startTime := time.Now()
	s := p.PodmanBase([]string{"logs", id})
	s.WaitWithDefaultTimeout()

	for {
		if time.Since(startTime) >= time.Duration(timeout)*time.Second {
			fmt.Printf("Container %s is not ready in %ds", id, timeout)
			return false
		}

		if strings.Contains(s.OutputToString(), expStr) {
			return true
		}
		time.Sleep(time.Duration(step) * time.Second)
		s = p.PodmanBase([]string{"logs", id})
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
	fields := strings.Fields(fmt.Sprintf("%s", s.Out.Contents()))
	return strings.Join(fields, " ")
}

// OutputToStringArray returns the output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) OutputToStringArray() []string {
	var results []string
	output := fmt.Sprintf("%s", s.Out.Contents())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// ErrorToString formats session stderr to string
func (s *PodmanSession) ErrorToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Err.Contents()))
	return strings.Join(fields, " ")
}

// ErrorToStringArray returns the stderr output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) ErrorToStringArray() []string {
	output := fmt.Sprintf("%s", s.Err.Contents())
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

//LineInOutputStartsWith returns true if a line in a
// session output starts with the supplied string
func (s *PodmanSession) LineInOuputStartsWith(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.HasPrefix(i, term) {
			return true
		}
	}
	return false
}

//LineInOutputContains returns true if a line in a
// session output starts with the supplied string
func (s *PodmanSession) LineInOutputContains(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.Contains(i, term) {
			return true
		}
	}
	return false
}

//LineInOutputContainsTag returns true if a line in the
// session's output contains the repo-tag pair as returned
// by podman-images(1).
func (s *PodmanSession) LineInOutputContainsTag(repo, tag string) bool {
	tagMap := tagOutputToMap(s.OutputToStringArray())
	for r, t := range tagMap {
		if repo == r && tag == t {
			return true
		}
	}
	return false
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

// WaitWithDefaultTimeout waits for process finished with defaultWaitTimeout
func (s *PodmanSession) WaitWithDefaultTimeout() {
	s.Wait(defaultWaitTimeout)
	fmt.Println("output:", s.OutputToString())
}

// CreateTempDirinTempDir create a temp dir with prefix podman_test
func CreateTempDirInTempDir() (string, error) {
	return ioutil.TempDir("", "podman_test")
}

// SystemExec is used to exec a system command to check its exit code or output
func SystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	session, err := gexec.Start(c, GinkgoWriter, GinkgoWriter)
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

//tagOutPutToMap parses each string in imagesOutput and returns
// a map of repo:tag pairs.  Notice, the first array item will
// be skipped as it's considered to be the header.
func tagOutputToMap(imagesOutput []string) map[string]string {
	m := make(map[string]string)
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
		m[tmp[0]] = tmp[1]
	}
	return m
}

//GetHostDistributionInfo returns a struct with its distribution name and version
func GetHostDistributionInfo() HostOS {
	f, err := os.Open(OSReleasePath)
	defer f.Close()
	if err != nil {
		return HostOS{}
	}

	l := bufio.NewScanner(f)
	host := HostOS{}
	host.Arch = runtime.GOARCH
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			host.Distribution = strings.Replace(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "", -1)
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			host.Version = strings.Replace(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "", -1)
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

//IsCommandAvaible check if command exist
func IsCommandAvailable(command string) bool {
	check := exec.Command("bash", "-c", strings.Join([]string{"command -v", command}, " "))
	err := check.Run()
	if err != nil {
		return false
	}
	return true
}

// WriteJsonFile write json format data to a json file
func WriteJsonFile(data []byte, filePath string) error {
	var jsonData map[string]interface{}
	json.Unmarshal(data, &jsonData)
	formatJson, _ := json.MarshalIndent(jsonData, "", "	")
	return ioutil.WriteFile(filePath, formatJson, 0644)
}

// Containerized check the podman command run inside container
func Containerized() bool {
	container := os.Getenv("container")
	if container != "" {
		return true
	}
	b, err := ioutil.ReadFile(ProcessOneCgroupPath)
	if err != nil {
		// shrug, if we cannot read that file, return false
		return false
	}
	if strings.Index(string(b), "docker") > -1 {
		return true
	}
	return false
}
