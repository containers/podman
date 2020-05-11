// +build !remoteclient

package integration

import (
	"os"
	"strconv"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod create", func() {
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman create infra container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		match, _ := check.GrepString(podID)
		Expect(match).To(BeTrue())
		Expect(len(check.OutputToStringArray())).To(Equal(1))

		check = podmanTest.Podman([]string{"ps", "-qa", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman start infra container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-qa", "--no-trunc", "--filter", "status=running"})
		check.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(check.OutputToStringArray())).To(Equal(1))
	})

	It("podman start infra container different image", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", BB})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		// If we use the default entry point, we should exit with no error
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman infra container namespaces", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podID)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--no-trunc", "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(check.OutputToStringArray())).To(Equal(2))
		Expect(check.OutputToStringArray()[0]).To(Equal(check.OutputToStringArray()[1]))

		check = podmanTest.Podman([]string{"ps", "-a", "--no-trunc", "--ns", "--format", "{{.IPC}} {{.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(check.OutputToStringArray())).To(Equal(2))
		Expect(check.OutputToStringArray()[0]).To(Equal(check.OutputToStringArray()[1]))
	})

	It("podman pod correctly sets up NetNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, fedoraMinimal, "curl", "localhost:80"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", fedoraMinimal, "curl", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pod correctly sets up IPCNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, fedoraMinimal, "/bin/sh", "-c", "'touch /dev/shm/hi'"})
		session.WaitWithDefaultTimeout()
		if session.ExitCode() != 0 {
			Skip("ShmDir not initialized, skipping...")
		}
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, fedoraMinimal, "/bin/sh", "-c", "'ls /dev/shm'"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("hi"))
	})

	It("podman pod correctly sets up PIDNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--name", "test-pod"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test-ctr", podID)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"top", "test-ctr", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		PIDs := check.OutputToStringArray()
		Expect(len(PIDs)).To(Equal(3))

		ctrPID, _ := strconv.Atoi(PIDs[1])
		infraPID, _ := strconv.Atoi(PIDs[2])
		Expect(ctrPID).To(BeNumerically("<", infraPID))
	})

	It("podman pod doesn't share PIDNS if requested to not", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net", "--name", "test-pod"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test-ctr", podID)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"top", "test-ctr", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		ctrTop := check.OutputToStringArray()

		check = podmanTest.Podman([]string{"top", podID[:12] + "-infra", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		infraTop := check.OutputToStringArray()

		ctrPID, _ := strconv.Atoi(ctrTop[1])
		infraPID, _ := strconv.Atoi(infraTop[1])
		Expect(ctrPID).To(Equal(infraPID))
	})

	It("podman pod container can override pod net NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--network", "bridge", nginx, "curl", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pod container can override pod pid NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "pid"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--pid", "host", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.Namespaces.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		check = podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray = check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Not(Equal(PID2)))
	})

	It("podman pod container can override pod not sharing pid", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--pid", "pod", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Equal(PID2))
	})

	It("podman pod container can override pod ipc NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--ipc", "host", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.IPC}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
		Expect(len(outputArray)).To(Equal(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Not(Equal(PID2)))
	})

	It("podman pod infra container deletion", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"ps", "-aq"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		infraID := session.OutputToString()

		session = podmanTest.Podman([]string{"rm", infraID})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"pod", "rm", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run in pod starts infra", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		infraID := result.OutputToString()

		result = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))

		Expect(result.OutputToString()).To(ContainSubstring(infraID))
	})

	It("podman start in pod starts infra", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		infraID := result.OutputToString()

		result = podmanTest.Podman([]string{"create", "--pod", podID, ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		ctrID := result.OutputToString()

		result = podmanTest.Podman([]string{"start", ctrID})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))

		Expect(result.OutputToString()).To(ContainSubstring(infraID))
	})

	It("podman run --add-host in pod", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		// verify we can add a host to the infra's /etc/hosts
		session = podmanTest.Podman([]string{"run", "--pod", podID, "--add-host", "foobar:127.0.0.1", BB, "ping", "-c", "1", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// verify we can see the other hosts of infra's /etc/hosts
		session = podmanTest.Podman([]string{"run", "--pod", podID, BB, "ping", "-c", "1", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run hostname is shared", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		// verify we can add a host to the infra's /etc/hosts
		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		hostname := session.OutputToString()

		infraName := podID[:12] + "-infra"
		// verify we can see the other hosts of infra's /etc/hosts
		session = podmanTest.Podman([]string{"inspect", infraName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})
})
