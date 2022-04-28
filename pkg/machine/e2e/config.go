package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	"github.com/containers/podman/v4/pkg/util"
	. "github.com/onsi/ginkgo" //nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gexec" //nolint:golint,stylecheck
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
	*gexec.Session
}

type machineTestBuilder struct {
	cmd          []string
	imagePath    string
	name         string
	names        []string
	podmanBinary string
	timeout      time.Duration
}
type qemuMachineInspectInfo struct {
	State machine.Status
	VM    qemu.MachineVM
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
			m.name = randomString(12)
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
func (mb *machineTestBuilder) toQemuInspectInfo() ([]qemuMachineInspectInfo, int, error) {
	args := []string{"machine", "inspect"}
	args = append(args, mb.names...)
	session, err := runWrapper(mb.podmanBinary, args, defaultTimeout, true)
	if err != nil {
		return nil, -1, err
	}
	mii := []qemuMachineInspectInfo{}
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

func (m *machineTestBuilder) init() {}

// randomString returns a string of given length composed of random characters
func randomString(n int) string {
	var randomLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = randomLetters[rand.Intn(len(randomLetters))]
	}
	return string(b)
}
