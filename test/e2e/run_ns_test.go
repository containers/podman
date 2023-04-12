package integration

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run ns", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman run pidns test", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("1"))

		session = podmanTest.Podman([]string{"run", "--pid=host", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(Equal("1")))

		session = podmanTest.Podman([]string{"run", "--pid=badpid", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run --cgroup private test", func() {
		session := podmanTest.Podman([]string{"run", "--cgroupns=private", fedoraMinimal, "cat", "/proc/self/cgroup"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		output := session.OutputToString()
		Expect(output).ToNot(ContainSubstring("slice"))
	})

	It("podman run ipcns test", func() {
		setup := SystemExec("ls", []string{"--inode", "-d", "/dev/shm"})
		Expect(setup).Should(Exit(0))
		hostShm := setup.OutputToString()

		session := podmanTest.Podman([]string{"run", "--ipc=host", fedoraMinimal, "ls", "--inode", "-d", "/dev/shm"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(hostShm))
	})

	It("podman run ipcns ipcmk host test", func() {
		setup := SystemExec("ipcmk", []string{"-M", "1024"})
		Expect(setup).Should(Exit(0))
		output := strings.Split(setup.OutputToString(), " ")
		ipc := output[len(output)-1]
		session := podmanTest.Podman([]string{"run", "--ipc=host", fedoraMinimal, "ipcs", "-m", "-i", ipc})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		setup = SystemExec("ipcrm", []string{"-m", ipc})
		Expect(setup).Should(Exit(0))
	})

	It("podman run ipcns ipcmk container test", func() {
		setup := podmanTest.Podman([]string{"run", "-d", "--name", "test1", fedoraMinimal, "sleep", "999"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"exec", "test1", "ipcmk", "-M", "1024"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := strings.Split(session.OutputToString(), " ")
		ipc := output[len(output)-1]
		session = podmanTest.Podman([]string{"run", "--ipc=container:test1", fedoraMinimal, "ipcs", "-m", "-i", ipc})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run bad ipc pid test", func() {
		session := podmanTest.Podman([]string{"run", "--ipc=badpid", fedoraMinimal, "bash", "-c", "echo $$"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run mounts fresh cgroup", func() {
		session := podmanTest.Podman([]string{"run", fedoraMinimal, "grep", "cgroup", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).ToNot(ContainSubstring(".."))
	})

	It("podman run --ipc=host --pid=host", func() {
		SkipIfRootlessCgroupsV1("Not supported for rootless + CgroupsV1")
		cmd := exec.Command("ls", "-l", "/proc/self/ns/pid")
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		fields := strings.Split(string(res), " ")
		hostPidNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		cmd = exec.Command("ls", "-l", "/proc/self/ns/ipc")
		res, err = cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		fields = strings.Split(string(res), " ")
		hostIpcNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		session := podmanTest.Podman([]string{"run", "--ipc=host", "--pid=host", ALPINE, "ls", "-l", "/proc/self/ns/pid"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fields = strings.Split(session.OutputToString(), " ")
		ctrPidNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		session = podmanTest.Podman([]string{"run", "--ipc=host", "--pid=host", ALPINE, "ls", "-l", "/proc/self/ns/ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		fields = strings.Split(session.OutputToString(), " ")
		ctrIpcNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		Expect(hostPidNS).To(Equal(ctrPidNS))
		Expect(hostIpcNS).To(Equal(ctrIpcNS))
	})

})
