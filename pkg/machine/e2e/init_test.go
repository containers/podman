package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/strongunits"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/sirupsen/logrus"
)

var _ = Describe("podman machine init", func() {
	cpus := runtime.NumCPU() / 2
	if cpus == 0 {
		cpus = 1
	}

	It("bad init", func() {
		i := initMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))

		reservedName := initMachine{}
		reservedNameSession, err := mb.setName(testProvider.VMType().String()).setCmd(&reservedName).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(reservedNameSession).To(Exit(125))
		Expect(reservedNameSession.errorToString()).To(ContainSubstring(fmt.Sprintf("cannot use %q", testProvider.VMType().String())))

		badName := "foobar"
		bm := basicMachine{}
		sysConn, err := mb.setCmd(bm.withPodmanCommand([]string{"system", "connection", "add", badName, "tcp://localhost:8000"})).run()
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			if _, rmErr := mb.setCmd(bm.withPodmanCommand([]string{"system", "connection", "rm", badName})).run(); rmErr != nil {
				logrus.Error(rmErr)
			}
		}()
		Expect(sysConn).To(Exit(0))

		bi := new(initMachine)
		want := fmt.Sprintf("system connection \"%s\" already exists", badName)
		badInit, berr := mb.setName(badName).setCmd(bi.withImage(mb.imagePath)).run()
		Expect(berr).ToNot(HaveOccurred())
		Expect(badInit).To(Exit(125))
		Expect(badInit.errorToString()).To(ContainSubstring(want))

		invalidName := "ab/cd"
		session, err = mb.setName(invalidName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring(`invalid name "ab/cd": names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*: invalid argument`))

		i.username = "-/a"
		session, err = mb.setName("").setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring(`invalid username "-/a": names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*: invalid argument`))

		// this comes in bytes
		memStat, err := mem.VirtualMemory()
		Expect(err).ToNot(HaveOccurred())
		total := strongunits.ToMib(strongunits.B(memStat.Total)) + 1024

		badMem := initMachine{}
		badMemSession, err := mb.setCmd(badMem.withMemory(uint(total))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(badMemSession).To(Exit(125))
	})

	It("simple init", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())

		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		if testProvider.VMType() != define.WSLVirt { // WSL hardware specs are hardcoded
			Expect(testMachine.Resources.CPUs).To(Equal(uint64(cpus)))
			Expect(testMachine.Resources.Memory).To(BeEquivalentTo(uint64(2048)))
		}
	})

	It("run playbook", func() {
		str := randomString()

		// ansible playbook file to create a text file containing a random string
		playbookContents := fmt.Sprintf(`- name: Simple podman machine example
  hosts: localhost
  tasks:
    - name: create a file
      ansible.builtin.copy:
        dest: ~/foobar.txt
        content: "%s\n"`, str)

		playbookPath := filepath.Join(GinkgoT().TempDir(), "playbook.yaml")

		// create the playbook file
		playbookFile, err := os.Create(playbookPath)
		Expect(err).ToNot(HaveOccurred())
		defer playbookFile.Close()

		// write the desired contents into the file
		_, err = playbookFile.WriteString(playbookContents)
		Expect(err).To(Not(HaveOccurred()))

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withRunPlaybook(playbookPath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// ensure the contents of the playbook file didn't change when getting copied
		ssh := new(sshMachine)
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"cat", "playbook.yaml"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToStringSlice()).To(Equal(strings.Split(playbookContents, "\n")))

		// wait until the playbook.service is done before checking to make sure the playbook was a success
		playbookFinished := false
		for range 900 {
			sshSession, err = mb.setName(name).setCmd(ssh.withSSHCommand([]string{"systemctl", "is-active", "playbook.service"})).run()
			Expect(err).ToNot(HaveOccurred())

			if sshSession.outputToString() == "inactive" {
				playbookFinished = true
				break
			}

			time.Sleep(10 * time.Millisecond)
		}

		if !playbookFinished {
			Fail("playbook.service did not finish")
		}

		// output the contents of the file generated by the playbook
		guestUser := "core"
		if isWSL() {
			guestUser = "user"
		}
		sshSession, err = mb.setName(name).setCmd(ssh.withSSHCommand([]string{"cat", fmt.Sprintf("/home/%s/foobar.txt", guestUser)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))

		// check its the same as the random number or string that we generated
		Expect(sshSession.outputToString()).To(Equal(str))
	})

	It("simple init with start", func() {
		i := initMachine{}
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(inspectBefore[0].Name).To(Equal(mb.names[0]))

		s := &startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		inspectAfter, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(inspectBefore).ToNot(BeEmpty())
		Expect(inspectAfter).ToNot(BeEmpty())
		Expect(inspectAfter[0].State).To(Equal(define.Running))

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
		session, err := mb.setCmd(i.withImage(mb.imagePath).withUsername(remoteUsername)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectBefore, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())

		Expect(inspectBefore).ToNot(BeEmpty())
		testMachine := inspectBefore[0]
		Expect(testMachine.Name).To(Equal(mb.names[0]))
		if testProvider.VMType() != define.WSLVirt { // memory and cpus something we cannot set with WSL
			Expect(testMachine.Resources.CPUs).To(Equal(uint64(cpus)))
			Expect(testMachine.Resources.Memory).To(BeEquivalentTo(uint64(2048)))
		}
		Expect(testMachine.SSHConfig.RemoteUsername).To(Equal(remoteUsername))

	})

	It("machine init with cpus, disk size, memory, timezone", func() {
		skipIfWSL("setting hardware resource numbers and timezone are not supported on WSL")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withCPUs(2).withDiskSize(102).withMemory(4096).withTimezone("Pacific/Honolulu")).run()
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
		if testProvider.VMType() == define.HyperVVirt {
			Skip("volumes are not supported on hyperv yet")
		}
		skipIfWSL("WSL volumes are much different.  This test will not pass as is")

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		_, err = os.CreateTemp(tmpDir, "example")
		Expect(err).ToNot(HaveOccurred())
		// Test long target path, see https://github.com/containers/podman/issues/22226
		mount := tmpDir + ":/very-long-test-mount-dir-path-more-than-thirty-six-bytes"
		defer func() { _ = utils.GuardedRemoveAll(tmpDir) }()

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withVolume(mount).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"ls /very-long-test-mount-dir-path-more-than-thirty-six-bytes"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring("example"))
	})

	It("machine init with ignition path", func() {
		skipIfWSL("Ignition is not compatible with WSL machines since they are not based on Fedora CoreOS")

		tmpDir, err := os.MkdirTemp("", "")
		defer func() { _ = utils.GuardedRemoveAll(tmpDir) }()
		Expect(err).ToNot(HaveOccurred())

		tmpFile, err := os.CreateTemp(tmpDir, "test-ignition-*.ign")
		Expect(err).ToNot(HaveOccurred())

		mockIgnitionContent := `{"ignition":{"version":"3.4.0"},"passwd":{"users":[{"name":"core"}]}}`

		_, err = tmpFile.WriteString(mockIgnitionContent)
		Expect(err).ToNot(HaveOccurred())

		err = tmpFile.Close()
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withIgnitionPath(tmpFile.Name())).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		configDir := filepath.Join(testDir, ".config", "containers", "podman", "machine", testProvider.VMType().String())

		// test that all required machine files are created
		fileExtensions := []string{".lock", ".json", ".ign"}
		for _, ext := range fileExtensions {
			filename := filepath.Join(configDir, fmt.Sprintf("%s%s", name, ext))

			_, err := os.Stat(filename)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("file %v does not exist", filename))
		}

		// enforce that the raw ignition is copied over verbatim
		createdIgn := filepath.Join(configDir, fmt.Sprintf("%s%s", name, ".ign"))
		contentWanted, err := os.ReadFile(tmpFile.Name())
		Expect(err).ToNot(HaveOccurred())
		contentGot, err := os.ReadFile(createdIgn)
		Expect(err).ToNot(HaveOccurred())
		Expect(contentWanted).To(Equal(contentGot), "The ignition file provided and the ignition file created do not match")
	})

	It("machine init rootless docker.sock check", func() {
		i := initMachine{}
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := &startMachine{}
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
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := &startMachine{}
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

	It("init should cleanup on failure", func() {
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()

		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.ConfigDir.Path}}")
		inspectSession, err := mb.setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		cfgpth := filepath.Join(inspectSession.outputToString(), fmt.Sprintf("%s.json", name))

		rm := rmMachine{}
		removeSession, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))

		// WSL does not use ignition
		if testProvider.VMType() != define.WSLVirt {
			// Bad ignition path - init fails
			i = new(initMachine)
			i.ignitionPath = "/bad/path"
			session, err = mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(125))

			imageSuffix := mb.imagePath[strings.LastIndex(mb.imagePath, "/")+1:]
			imgPath := filepath.Join(testDir, ".local", "share", "containers", "podman", "machine", "qemu", mb.name+"_"+imageSuffix)
			_, err = os.Stat(imgPath)
			Expect(err).To(HaveOccurred())

			cfgDir := filepath.Join(testDir, ".config", "containers", "podman", "machine", testProvider.VMType().String())
			_, err = os.Stat(cfgpth)
			Expect(err).To(HaveOccurred())

			ignPath := filepath.Join(cfgDir, mb.name+".ign")
			_, err = os.Stat(ignPath)
			Expect(err).To(HaveOccurred())
		}
	})

	It("verify a podman 4 config does not break podman 5", func() {
		vmName := "foobar-machine"
		configDir := filepath.Join(testDir, ".config", "containers", "podman", "machine", testProvider.VMType().String())
		if err := os.MkdirAll(configDir, 0755); err != nil {
			Expect(err).ToNot(HaveOccurred())
		}
		f, err := os.Create(filepath.Join(configDir, fmt.Sprintf("%s.json", vmName)))
		Expect(err).ToNot(HaveOccurred())
		if _, err := f.Write(p4Config); err != nil {
			Expect(err).ToNot(HaveOccurred())
		}
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		// At this point we have a p4 config in the config dir
		// podman machine list should emit a "soft" error but complete
		list := new(listMachine)
		firstList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.errorToString()).To(ContainSubstring("incompatible machine config"))

		// podman machine inspect should fail because we are
		// trying to work with the incompatible machine json
		ins := inspectMachine{}
		inspectShouldFail, err := mb.setName(vmName).setCmd(&ins).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectShouldFail).To(Exit(125))
		Expect(inspectShouldFail.errorToString()).To(ContainSubstring("incompatible machine config"))

		// We should be able to init with a bad config present
		i := new(initMachine)
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// We should still be able to ls
		secondList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).Should(Exit(0))

		// And inspecting the valid machine should not error
		inspectShouldPass, err := mb.setName(name).setCmd(&ins).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectShouldPass).To(Exit(0))
	})

	It("machine init with rosetta=true", func() {
		skipIfVmtype(define.QemuVirt, "Test is only for AppleHv")
		skipIfVmtype(define.WSLVirt, "Test is only for AppleHv")
		skipIfVmtype(define.HyperVVirt, "Test is only for AppleHv")
		skipIfVmtype(define.LibKrun, "Test is only for AppleHv")
		if runtime.GOARCH != "arm64" {
			Skip("Test is only for AppleHv with arm64 architecture")
		}

		i := initMachine{}
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := &startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Rosetta}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))

		mnt := sshMachine{}
		mntSession, err := mb.setName(name).setCmd(mnt.withSSHCommand([]string{"ls -d /mnt/rosetta"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(mntSession).To(Exit(0))
		Expect(mntSession.outputToString()).To(ContainSubstring("/mnt/rosetta"))

		proc := sshMachine{}
		procSession, err := mb.setName(name).setCmd(proc.withSSHCommand([]string{"ls -d /proc/sys/fs/binfmt_misc/rosetta"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(procSession).To(Exit(0))
		Expect(procSession.outputToString()).To(ContainSubstring("/proc/sys/fs/binfmt_misc/rosetta"))

		proc2 := sshMachine{}
		proc2Session, err := mb.setName(name).setCmd(proc2.withSSHCommand([]string{"ls -d /proc/sys/fs/binfmt_misc/qemu-x86_64"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(proc2Session.ExitCode()).To(Equal(2))
	})

	It("machine init with rosetta=false", func() {
		skipIfVmtype(define.QemuVirt, "Test is only for AppleHv")
		skipIfVmtype(define.WSLVirt, "Test is only for AppleHv")
		skipIfVmtype(define.HyperVVirt, "Test is only for AppleHv")
		skipIfVmtype(define.LibKrun, "Test is only for AppleHv")
		if runtime.GOARCH != "arm64" {
			Skip("Test is only for AppleHv with arm64 architecture")
		}
		configDir := filepath.Join(testDir, ".config", "containers")
		err := os.MkdirAll(configDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		err = os.WriteFile(filepath.Join(configDir, "containers.conf"), rosettaConfig, 0644)
		Expect(err).ToNot(HaveOccurred())

		i := initMachine{}
		name := randomString()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := &startMachine{}
		ssession, err := mb.setCmd(s).setTimeout(time.Minute * 10).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ssession).Should(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Rosetta}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("false"))

		mnt := sshMachine{}
		mntSession, err := mb.setName(name).setCmd(mnt.withSSHCommand([]string{"ls -d /mnt/rosetta"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(mntSession.ExitCode()).To(Equal(2))

		proc := sshMachine{}
		procSession, err := mb.setName(name).setCmd(proc.withSSHCommand([]string{"ls -d /proc/sys/fs/binfmt_misc/rosetta"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(procSession.ExitCode()).To(Equal(2))

		proc2 := sshMachine{}
		proc2Session, err := mb.setName(name).setCmd(proc2.withSSHCommand([]string{"ls -d /proc/sys/fs/binfmt_misc/qemu-x86_64"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(proc2Session.outputToString()).To(ContainSubstring("/proc/sys/fs/binfmt_misc/qemu-x86_64"))
	})
})

var p4Config = []byte(`{
 "ConfigPath": {
  "Path": "/home/baude/.config/containers/podman/machine/qemu/podman-machine-default.json"
 },
 "CmdLine": [
  "/usr/bin/qemu-system-x86_64",
  "-accel",
  "kvm",
  "-cpu",
  "host",
  "-m",
  "2048",
  "-smp",
  "12",
  "-fw_cfg",
  "name=opt/com.coreos/config,file=/home/baude/.config/containers/podman/machine/qemu/podman-machine-default.ign",
  "-qmp",
  "unix:/run/user/1000/podman/qmp_podman-machine-default.sock,server=on,wait=off",
  "-netdev",
  "socket,id=vlan,fd=3",
  "-device",
  "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee",
  "-device",
  "virtio-serial",
  "-chardev",
  "socket,path=/run/user/1000/podman/podman-machine-default_ready.sock,server=on,wait=off,id=apodman-machine-default_ready",
  "-device",
  "virtserialport,chardev=apodman-machine-default_ready,name=org.fedoraproject.port.0",
  "-pidfile",
  "/run/user/1000/podman/podman-machine-default_vm.pid",
  "-virtfs",
  "local,path=/home/baude,mount_tag=vol0,security_model=none",
  "-drive",
  "if=virtio,file=/home/baude/.local/share/containers/podman/machine/qemu/podman-machine-default_fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2"
 ],
 "Rootful": false,
 "UID": 1000,
 "HostUserModified": false,
 "IgnitionFilePath": {
  "Path": "/home/baude/.config/containers/podman/machine/qemu/podman-machine-default.ign"
 },
 "ImageStream": "testing",
 "ImagePath": {
  "Path": "/home/baude/.local/share/containers/podman/machine/qemu/podman-machine-default_fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2"
 },
 "Mounts": [
  {
   "ReadOnly": false,
   "Source": "/home/baude",
   "Tag": "vol0",
   "Target": "/home/baude",
   "Type": "9p"
  }
 ],
 "Name": "podman-machine-default",
 "PidFilePath": {
  "Path": "/run/user/1000/podman/podman-machine-default_proxy.pid"
 },
 "VMPidFilePath": {
  "Path": "/run/user/1000/podman/podman-machine-default_vm.pid"
 },
 "QMPMonitor": {
  "Address": {
   "Path": "/run/user/1000/podman/qmp_podman-machine-default.sock"
  },
  "Network": "unix",
  "Timeout": 2000000000
 },
 "ReadySocket": {
  "Path": "/run/user/1000/podman/podman-machine-default_ready.sock"
 },
 "CPUs": 12,
 "DiskSize": 100,
 "Memory": 2048,
 "USBs": [],
 "IdentityPath": "/home/baude/.local/share/containers/podman/machine/machine",
 "Port": 38419,
 "RemoteUsername": "core",
 "Starting": false,
 "Created": "2024-02-08T10:34:14.067604999-06:00",
 "LastUp": "0001-01-01T00:00:00Z"
}
`)

var rosettaConfig = []byte(`
[machine]
rosetta=false
`)
