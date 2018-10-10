package integration

import (
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod create", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
		podmanTest.RestoreArtifact(infra)
	})

	AfterEach(func() {
		podmanTest.CleanupPod()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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

		check := podmanTest.Podman([]string{"ps", "-a", "--no-trunc", "--ns", "--format", "{{.IPC}} {{.NET}}"})
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

		podmanTest.RestoreArtifact(nginx)
		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		podmanTest.RestoreArtifact(fedoraMinimal)
		session = podmanTest.Podman([]string{"run", "--pod", podID, fedoraMinimal, "curl", "localhost:80"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", fedoraMinimal, "curl", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman pod correctly sets up IPCNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		podmanTest.RestoreArtifact(fedoraMinimal)
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

		podmanTest.RestoreArtifact(nginx)
		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		podmanTest.RestoreArtifact(fedoraMinimal)
		session = podmanTest.Podman([]string{"run", "--pod", podID, "--network", "bridge", fedoraMinimal, "curl", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
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

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		outputArray := check.OutputToStringArray()
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
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"pod", "rm", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
