// +build !remoteclient

package integration

import (
	"net"
	"os"
	"os/exec"

	"github.com/containers/libpod/pkg/criu"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getRunString(input []string) []string {
	// CRIU does not work with seccomp correctly on RHEL7 : seccomp=unconfined
	runString := []string{"run", "-it", "--security-opt", "seccomp=unconfined", "-d", "--ip", GetRandomIPAddress()}
	return append(runString, input...)
}

var _ = Describe("Podman checkpoint", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
		// Check if the runtime implements checkpointing. Currently only
		// runc's checkpoint/restore implementation is supported.
		cmd := exec.Command(podmanTest.OCIRuntime, "checkpoint", "-h")
		if err := cmd.Start(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}
		if err := cmd.Wait(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}

		if !criu.CheckForCriu() {
			Skip("CRIU is missing or too old.")
		}
		// Only Fedora 29 and newer has a new enough selinux-policy and
		// container-selinux package to support CRIU in correctly
		// restoring threaded processes
		hostInfo := podmanTest.Host
		if hostInfo.Distribution == "fedora" && hostInfo.Version < "29" {
			Skip("Checkpoint/Restore with SELinux only works on Fedora >= 29")
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman checkpoint bogus container", func() {
		session := podmanTest.Podman([]string{"container", "checkpoint", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman restore bogus container", func() {
		session := podmanTest.Podman([]string{"container", "restore", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman checkpoint a running container by id", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman checkpoint a running container by name", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman pause a checkpointed container by id", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("podman checkpoint latest running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1.ExitCode()).To(Equal(0))

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps.ExitCode()).To(Equal(0))
		Expect(ps.LineInOutputContains(session1.OutputToString())).To(BeTrue())
		Expect(ps.LineInOutputContains(session2.OutputToString())).To(BeFalse())

		result = podmanTest.Podman([]string{"container", "restore", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint all running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1.ExitCode()).To(Equal(0))

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps.ExitCode()).To(Equal(0))
		Expect(ps.LineInOutputContains(session1.OutputToString())).To(BeFalse())
		Expect(ps.LineInOutputContains(session2.OutputToString())).To(BeFalse())

		result = podmanTest.Podman([]string{"container", "restore", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint container with established tcp connections", func() {
		localRunString := getRunString([]string{redis})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		IP := podmanTest.Podman([]string{"inspect", "-l", "--format={{.NetworkSettings.IPAddress}}"})
		IP.WaitWithDefaultTimeout()
		Expect(IP.ExitCode()).To(Equal(0))

		// Open a network connection to the redis server
		conn, err := net.Dial("tcp", IP.OutputToString()+":6379")
		if err != nil {
			os.Exit(1)
		}
		// This should fail as the container has established TCP connections
		result := podmanTest.Podman([]string{"container", "checkpoint", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Now it should work thanks to "--tcp-established"
		result = podmanTest.Podman([]string{"container", "checkpoint", "-l", "--tcp-established"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore should fail as the checkpoint image contains established TCP connections
		result = podmanTest.Podman([]string{"container", "restore", "-l"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Now it should work thanks to "--tcp-established"
		result = podmanTest.Podman([]string{"container", "restore", "-l", "--tcp-established"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		conn.Close()
	})

	It("podman checkpoint with --leave-running", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		// Checkpoint container, but leave it running
		result := podmanTest.Podman([]string{"container", "checkpoint", "--leave-running", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		// Make sure it is still running
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Stop the container
		result = podmanTest.Podman([]string{"container", "stop", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore the stopped container from the previous checkpoint
		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint and restore container with same IP", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		IPBefore := podmanTest.Podman([]string{"inspect", "-l", "--format={{.NetworkSettings.IPAddress}}"})
		IPBefore.WaitWithDefaultTimeout()
		Expect(IPBefore.ExitCode()).To(Equal(0))

		MACBefore := podmanTest.Podman([]string{"inspect", "-l", "--format={{.NetworkSettings.MacAddress}}"})
		MACBefore.WaitWithDefaultTimeout()
		Expect(MACBefore.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		IPAfter := podmanTest.Podman([]string{"inspect", "-l", "--format={{.NetworkSettings.IPAddress}}"})
		IPAfter.WaitWithDefaultTimeout()
		Expect(IPAfter.ExitCode()).To(Equal(0))

		MACAfter := podmanTest.Podman([]string{"inspect", "-l", "--format={{.NetworkSettings.MacAddress}}"})
		MACAfter.WaitWithDefaultTimeout()
		Expect(MACAfter.ExitCode()).To(Equal(0))

		// Check that IP address did not change between checkpointing and restoring
		Expect(IPBefore.OutputToString()).To(Equal(IPAfter.OutputToString()))

		// Check that MAC address did not change between checkpointing and restoring
		Expect(MACBefore.OutputToString()).To(Equal(MACAfter.OutputToString()))

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	// This test does the same steps which are necessary for migrating
	// a container from one host to another
	It("podman checkpoint container with export (migration)", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{"container", "checkpoint", "-l", "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container the first time with different name.
		// Using '--ignore-static-ip' as for parallel test runs
		// each containers gets a random IP address via '--ip'.
		// '--ignore-static-ip' tells the restore to use the next
		// available IP address.
		// First restore the container with a new name/ID to make
		// sure nothing in the restored container depends on the
		// original container.
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName, "-n", "restore_again", "--ignore-static-ip"})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and restore container with root file-system changes", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", "-l", "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", "-l", "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", "-l", "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during restore", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", "-l", "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", "-l", "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "--ignore-rootfs", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", "-l", "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(1))
		Expect(result.ErrorToString()).To(ContainSubstring("cat: can't open '/test.output': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during checkpoint", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", "-l", "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", "--ignore-rootfs", "-l", "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", "-l", "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(1))
		Expect(result.ErrorToString()).To(ContainSubstring("cat: can't open '/test.output': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and run exec in restored container", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Checkpoint the container
		result := podmanTest.Podman([]string{"container", "checkpoint", "-l", "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Exec in the container
		result = podmanTest.Podman([]string{"exec", "-l", "/bin/sh", "-c", "echo " + cid + " > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"exec", "-l", "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
})
