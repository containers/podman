package e2e_test

import (
	"os"
	"strconv"
	"time"

	"github.com/containers/podman/v4/pkg/machine"

	. "github.com/onsi/ginkgo"
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

	It("bad init name", func() {
		i := initMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(125))
	})
	It("simple init", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).To(BeNil())
		Expect(ec).To(BeZero())

		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		Expect(testMachine.Resources.CPUs).To(Equal(uint64(1)))
		Expect(testMachine.Resources.Memory).To(Equal(uint64(2048)))

	})

	It("simple init with start", func() {
		i := initMachine{}
		session, err := mb.setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(ec).To(BeZero())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(err).To(BeNil())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(inspectBefore[0].Name).To(Equal(mb.names[0]))

		s := startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).To(BeNil())
		Expect(ssession).Should(Exit(0))

		inspectAfter, ec, err := mb.toQemuInspectInfo()
		Expect(err).To(BeNil())
		Expect(ec).To(BeZero())
		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		Expect(len(inspectAfter)).To(BeNumerically(">", 0))
		Expect(inspectAfter[0].State).To(Equal(machine.Running))
	})

	It("simple init with username", func() {
		i := new(initMachine)
		remoteUsername := "remoteuser"
		session, err := mb.setCmd(i.withImagePath(mb.imagePath).withUsername(remoteUsername)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).To(BeNil())
		Expect(ec).To(BeZero())

		Expect(len(inspectBefore)).To(BeNumerically(">", 0))
		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		Expect(testMachine.Resources.CPUs).To(Equal(uint64(1)))
		Expect(testMachine.Resources.Memory).To(Equal(uint64(2048)))
		Expect(testMachine.SSHConfig.RemoteUsername).To((Equal(remoteUsername)))

	})

	It("machine init with cpus, disk size, memory, timezone", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withCPUs(2).withDiskSize(102).withMemory(4096).withTimezone("Pacific/Honolulu")).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).To(BeNil())
		Expect(startSession).To(Exit(0))

		sshCPU := sshMachine{}
		CPUsession, err := mb.setName(name).setCmd(sshCPU.withSSHComand([]string{"lscpu", "|", "grep", "\"CPU(s):\"", "|", "head", "-1"})).run()
		Expect(err).To(BeNil())
		Expect(CPUsession).To(Exit(0))
		Expect(CPUsession.outputToString()).To(ContainSubstring("2"))

		sshDisk := sshMachine{}
		diskSession, err := mb.setName(name).setCmd(sshDisk.withSSHComand([]string{"sudo", "fdisk", "-l", "|", "grep", "Disk"})).run()
		Expect(err).To(BeNil())
		Expect(diskSession).To(Exit(0))
		Expect(diskSession.outputToString()).To(ContainSubstring("102 GiB"))

		sshMemory := sshMachine{}
		memorySession, err := mb.setName(name).setCmd(sshMemory.withSSHComand([]string{"cat", "/proc/meminfo", "|", "grep", "-i", "'memtotal'", "|", "grep", "-o", "'[[:digit:]]*'"})).run()
		Expect(err).To(BeNil())
		Expect(memorySession).To(Exit(0))
		foundMemory, err := strconv.Atoi(memorySession.outputToString())
		Expect(err).To(BeNil())
		Expect(foundMemory).To(BeNumerically(">", 3800000))
		Expect(foundMemory).To(BeNumerically("<", 4200000))

		sshTimezone := sshMachine{}
		timezoneSession, err := mb.setName(name).setCmd(sshTimezone.withSSHComand([]string{"date"})).run()
		Expect(err).To(BeNil())
		Expect(timezoneSession).To(Exit(0))
		Expect(timezoneSession.outputToString()).To(ContainSubstring("HST"))
	})

	It("machine init with volume", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).To(BeNil())
		_, err = os.CreateTemp(tmpDir, "example")
		Expect(err).To(BeNil())
		mount := tmpDir + ":/testmountdir"
		defer os.RemoveAll(tmpDir)

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withVolume(mount)).run()
		Expect(err).To(BeNil())
		Expect(session).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).To(BeNil())
		Expect(startSession).To(Exit(0))

		ssh2 := sshMachine{}
		sshSession2, err := mb.setName(name).setCmd(ssh2.withSSHComand([]string{"ls /testmountdir"})).run()
		Expect(err).To(BeNil())
		Expect(sshSession2).To(Exit(0))
		Expect(sshSession2.outputToString()).To(ContainSubstring("example"))
	})

})
