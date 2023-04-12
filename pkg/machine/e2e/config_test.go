package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var originalHomeDir = os.Getenv("HOME")

const (
	defaultTimeout time.Duration = 90 * time.Second
)

type machineCommand interface {
	buildCmd(m *machineTestBuilder) []string
}

type MachineTestBuilder interface {
	setName(string) *MachineTestBuilder
	setCmd(mc machineCommand) *MachineTestBuilder
	setTimeout(duration time.Duration) *MachineTestBuilder
	run() (*machineSession, error)
}
type machineSession struct {
	*Session
}

type machineTestBuilder struct {
	cmd          []string
	imagePath    string
	name         string
	names        []string
	podmanBinary string
	timeout      time.Duration
}

// waitWithTimeout waits for a command to complete for a given
// number of seconds
func (ms *machineSession) waitWithTimeout(timeout time.Duration) {
	Eventually(ms, timeout).Should(Exit())
	os.Stdout.Sync()
	os.Stderr.Sync()
}

func (ms *machineSession) Bytes() []byte {
	return []byte(ms.outputToString())
}

func (ms *machineSession) outputToStringSlice() []string {
	var results []string
	output := string(ms.Out.Contents())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// outputToString returns the output from a session in string form
func (ms *machineSession) outputToString() string {
	if ms == nil || ms.Out == nil || ms.Out.Contents() == nil {
		return ""
	}

	fields := strings.Fields(string(ms.Out.Contents()))
	return strings.Join(fields, " ")
}

// errorToString returns the error output from a session in string form
func (ms *machineSession) errorToString() string {
	if ms == nil || ms.Err == nil || ms.Err.Contents() == nil {
		return ""
	}
	return string(ms.Err.Contents())
}

// newMB constructor for machine test builders
func newMB() (*machineTestBuilder, error) {
	mb := machineTestBuilder{
		timeout: defaultTimeout,
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	mb.podmanBinary = filepath.Join(cwd, "../../../bin/podman-remote")
	if os.Getenv("PODMAN_BINARY") != "" {
		mb.podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	return &mb, nil
}

// setName sets the name of the virtuaql machine for the command
func (m *machineTestBuilder) setName(name string) *machineTestBuilder {
	m.name = name
	return m
}

// setCmd takes a machineCommand struct and assembles a cmd line
// representation of the podman machine command
func (m *machineTestBuilder) setCmd(mc machineCommand) *machineTestBuilder {
	// If no name for the machine exists, we set a random name.
	if !util.StringInSlice(m.name, m.names) {
		if len(m.name) < 1 {
			m.name = randomString()
		}
		m.names = append(m.names, m.name)
	}
	m.cmd = mc.buildCmd(m)
	return m
}

func (m *machineTestBuilder) setTimeout(timeout time.Duration) *machineTestBuilder {
	m.timeout = timeout
	return m
}

// toQemuInspectInfo is only for inspecting qemu machines.  Other providers will need
// to make their own.
func (m *machineTestBuilder) toQemuInspectInfo() ([]machine.InspectInfo, int, error) {
	args := []string{"machine", "inspect"}
	args = append(args, m.names...)
	session, err := runWrapper(m.podmanBinary, args, defaultTimeout, true)
	if err != nil {
		return nil, -1, err
	}
	mii := []machine.InspectInfo{}
	err = json.Unmarshal(session.Bytes(), &mii)
	return mii, session.ExitCode(), err
}

func (m *machineTestBuilder) runWithoutWait() (*machineSession, error) {
	return runWrapper(m.podmanBinary, m.cmd, m.timeout, false)
}

func (m *machineTestBuilder) run() (*machineSession, error) {
	return runWrapper(m.podmanBinary, m.cmd, m.timeout, true)
}

func runWrapper(podmanBinary string, cmdArgs []string, timeout time.Duration, wait bool) (*machineSession, error) {
	if len(os.Getenv("DEBUG")) > 0 {
		cmdArgs = append([]string{"--log-level=debug"}, cmdArgs...)
	}
	fmt.Println(podmanBinary + " " + strings.Join(cmdArgs, " "))
	c := exec.Command(podmanBinary, cmdArgs...)
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("Unable to start session: %q", err))
		return nil, err
	}
	ms := machineSession{session}
	if wait {
		ms.waitWithTimeout(timeout)
		fmt.Println("output:", ms.outputToString())
	}
	return &ms, nil
}

// randomString returns a string of given length composed of random characters
func randomString() string {
	return stringid.GenerateRandomID()[0:12]
}
