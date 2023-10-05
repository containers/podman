package e2e_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine init", func() {
	var (
		mb      *machineTestBuilder
		testDir string
	)

	BeforeEach(func() {
		testDir, mb = setup()
	})
	AfterEach(func() {
		teardown(originalHomeDir, testDir, mb)
	})

	cpus := runtime.NumCPU() / 2
	if cpus == 0 {
		cpus = 1
	}

	It("bad init name", func() {
		i := initMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})
	It("simple init", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())

		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		if testProvider.VMType() == machine.WSLVirt { // WSL hardware specs are hardcoded
			Expect(testMachine.Resources.CPUs).To(Equal(uint64(cpus)))
			Expect(testMachine.Resources.Memory).To(Equal(uint64(2048)))
		}
	})

	It("simple init with start", func() {
		i := initMachine{}
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(inspectBefore[0].Name).To(Equal(mb.names[0]))

		s := startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		inspectAfter, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(inspectAfter).ToNot(BeEmpty())
		Expect(inspectAfter[0].State).To(Equal(machine.Running))

		if isWSL() { // WSL does not use FCOS
			return
		}

		// check to see that zincati is masked
		sshDisk := sshMachine{}
		zincati, err := mb.setCmd(sshDisk.withSSHCommand([]string{"sudo", "systemctl", "is-enabled", "zincati"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(zincati.outputToString()).To(ContainSubstring("disabled"))
	})

	It("simple init with username", func() {
		i := new(initMachine)
		remoteUsername := "remoteuser"
		session, err := mb.setCmd(i.withImagePath(mb.imagePath).withUsername(remoteUsername)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())

		Expect(inspectBefore).ToNot(BeEmpty())
		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		if testProvider.VMType() != machine.WSLVirt { // memory and cpus something we cannot set with WSL
			Expect(testMachine.Resources.CPUs).To(Equal(uint64(cpus)))
			Expect(testMachine.Resources.Memory).To(Equal(uint64(2048)))
		}
		Expect(testMachine.SSHConfig.RemoteUsername).To(Equal(remoteUsername))

	})

	It("machine init with cpus, disk size, memory, timezone", func() {
		skipIfWSL("setting hardware resource numbers and timezone are not supported on WSL")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withCPUs(2).withDiskSize(102).withMemory(4096).withTimezone("Pacific/Honolulu")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshCPU := sshMachine{}
		CPUsession, err := mb.setName(name).setCmd(sshCPU.withSSHCommand([]string{"lscpu", "|", "grep", "\"CPU(s):\"", "|", "head", "-1"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(CPUsession).To(Exit(0))
		Expect(CPUsession.outputToString()).To(ContainSubstring("2"))

		sshDisk := sshMachine{}
		diskSession, err := mb.setName(name).setCmd(sshDisk.withSSHCommand([]string{"sudo", "fdisk", "-l", "|", "grep", "Disk"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(diskSession).To(Exit(0))
		Expect(diskSession.outputToString()).To(ContainSubstring("102 GiB"))

		sshMemory := sshMachine{}
		memorySession, err := mb.setName(name).setCmd(sshMemory.withSSHCommand([]string{"cat", "/proc/meminfo", "|", "grep", "-i", "'memtotal'", "|", "grep", "-o", "'[[:digit:]]*'"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(memorySession).To(Exit(0))
		foundMemory, err := strconv.Atoi(memorySession.outputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(foundMemory).To(BeNumerically(">", 3800000))
		Expect(foundMemory).To(BeNumerically("<", 4200000))

		sshTimezone := sshMachine{}
		timezoneSession, err := mb.setName(name).setCmd(sshTimezone.withSSHCommand([]string{"date"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(timezoneSession).To(Exit(0))
		Expect(timezoneSession.outputToString()).To(ContainSubstring("HST"))
	})

	It("machine init with volume", func() {
		if testProvider.VMType() == machine.HyperVVirt {
			Skip("volumes are not supported on hyperv yet")
		}
		skipIfWSL("WSL volumes are much different.  This test will not pass as is")

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		_, err = os.CreateTemp(tmpDir, "example")
		Expect(err).ToNot(HaveOccurred())
		mount := tmpDir + ":/testmountdir"
		defer func() { _ = machine.GuardedRemoveAll(tmpDir) }()

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withVolume(mount).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"ls /testmountdir"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring("example"))
	})

	It("machine init rootless docker.sock check", func() {
		i := initMachine{}
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		ssh2 := sshMachine{}
		sshSession2, err := mb.setName(name).setCmd(ssh2.withSSHCommand([]string{"readlink /var/run/docker.sock"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession2).To(Exit(0))

		output := strings.TrimSpace(sshSession2.outputToString())
		Expect(output).To(HavePrefix("/run/user"))
		Expect(output).To(HaveSuffix("/podman/podman.sock"))

	})

	It("machine init rootful with docker.sock check", func() {
		i := initMachine{}
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Rootful}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))

		ssh2 := sshMachine{}
		sshSession2, err := mb.setName(name).setCmd(ssh2.withSSHCommand([]string{"readlink /var/run/docker.sock"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession2).To(Exit(0))
		output := strings.TrimSpace(sshSession2.outputToString())
		Expect(output).To(Equal("/run/podman/podman.sock"))
	})

	It("init with user mode networking ", func() {
		if testProvider.VMType() != machine.WSLVirt {
			Skip("test is only supported by WSL")
		}
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withUserModeNetworking(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.UserModeNetworking}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))
	})
})
