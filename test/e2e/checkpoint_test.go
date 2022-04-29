package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/checkpoint-restore/go-criu/v5/stats"
	"github.com/containers/podman/v4/pkg/checkpoint/crutils"
	"github.com/containers/podman/v4/pkg/criu"
	"github.com/containers/podman/v4/pkg/domain/entities"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/podman/v4/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		SkipIfRootless("checkpoint not supported in rootless mode")
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		// Check if the runtime implements checkpointing. Currently only
		// runc's checkpoint/restore implementation is supported.
		cmd := exec.Command(podmanTest.OCIRuntime, "checkpoint", "--help")
		if err := cmd.Start(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}
		if err := cmd.Wait(); err != nil {
			Skip("OCI runtime does not support checkpoint/restore")
		}

		if !criu.CheckForCriu(criu.MinCriuVersion) {
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
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		// Check if none of the checkpoint/restore specific information is displayed
		// for newly started containers.
		inspect := podmanTest.Podman([]string{"inspect", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectOut := inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State).To(HaveField("CheckpointPath", ""))
		Expect(inspectOut[0].State).To(HaveField("CheckpointLog", ""))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			"--keep",
			cid,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// For a checkpointed container we expect the checkpoint related information
		// to be populated.
		inspect = podmanTest.Podman([]string{"inspect", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeTrue(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State.CheckpointPath).To(ContainSubstring("userdata/checkpoint"))
		Expect(inspectOut[0].State.CheckpointLog).To(ContainSubstring("userdata/dump.log"))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))

		result = podmanTest.Podman([]string{
			"container",
			"restore",
			"--keep",
			cid,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		inspect = podmanTest.Podman([]string{"inspect", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Restored).To(BeTrue(), ".State.Restored")
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.CheckpointPath).To(ContainSubstring("userdata/checkpoint"))
		Expect(inspectOut[0].State.CheckpointLog).To(ContainSubstring("userdata/dump.log"))
		Expect(inspectOut[0].State.RestoreLog).To(ContainSubstring("userdata/restore.log"))

		result = podmanTest.Podman([]string{
			"container",
			"stop",
			"--timeout",
			"0",
			cid,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{
			"container",
			"start",
			cid,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Stopping and starting the container should remove all checkpoint
		// related information from inspect again.
		inspect = podmanTest.Podman([]string{"inspect", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		inspectOut = inspect.InspectContainerToJSON()
		Expect(inspectOut[0].State.Checkpointed).To(BeFalse(), ".State.Checkpointed")
		Expect(inspectOut[0].State.Restored).To(BeFalse(), ".State.Restored")
		Expect(inspectOut[0].State).To(HaveField("CheckpointPath", ""))
		Expect(inspectOut[0].State).To(HaveField("CheckpointLog", ""))
		Expect(inspectOut[0].State).To(HaveField("RestoreLog", ""))
	})

	It("podman checkpoint a running container by name", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman pause a checkpointed container by id", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"pause", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(2))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "-t", "1", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

	})

	It("podman checkpoint latest running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "second"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(session1.OutputToString()))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{"container", "restore", "second"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint all running container", func() {
		localRunString := getRunString([]string{"--name", "first", ALPINE, "top"})
		session1 := podmanTest.Podman(localRunString)
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		localRunString = getRunString([]string{"--name", "second", ALPINE, "top"})
		session2 := podmanTest.Podman(localRunString)
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		ps := podmanTest.Podman([]string{"ps", "-q", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session1.OutputToString())))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{"container", "restore", "-a"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint container with established tcp connections", func() {
		// Broken on Ubuntu.
		SkipIfNotFedora()
		localRunString := getRunString([]string{redis})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		if !WaitContainerReady(podmanTest, cid, "Ready to accept connections", 20, 1) {
			Fail("Container failed to get ready")
		}

		IP := podmanTest.Podman([]string{"inspect", cid, "--format={{.NetworkSettings.IPAddress}}"})
		IP.WaitWithDefaultTimeout()
		Expect(IP).Should(Exit(0))

		// Open a network connection to the redis server
		conn, err := net.DialTimeout("tcp4", IP.OutputToString()+":6379", time.Duration(3)*time.Second)
		Expect(err).To(BeNil())

		// This should fail as the container has established TCP connections
		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Now it should work thanks to "--tcp-established"
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "--tcp-established"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore should fail as the checkpoint image contains established TCP connections
		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Now it should work thanks to "--tcp-established"
		result = podmanTest.Podman([]string{"container", "restore", cid, "--tcp-established"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		conn.Close()
	})

	It("podman checkpoint with --leave-running", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		// Checkpoint container, but leave it running
		result := podmanTest.Podman([]string{"container", "checkpoint", "--leave-running", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		// Make sure it is still running
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Stop the container
		result = podmanTest.Podman([]string{"container", "stop", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		// Restore the stopped container from the previous checkpoint
		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint and restore container with same IP", func() {
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		IPBefore := podmanTest.Podman([]string{"inspect", "test_name", "--format={{.NetworkSettings.IPAddress}}"})
		IPBefore.WaitWithDefaultTimeout()
		Expect(IPBefore).Should(Exit(0))

		MACBefore := podmanTest.Podman([]string{"inspect", "test_name", "--format={{.NetworkSettings.MacAddress}}"})
		MACBefore.WaitWithDefaultTimeout()
		Expect(MACBefore).Should(Exit(0))

		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "test_name"})
		result.WaitWithDefaultTimeout()

		IPAfter := podmanTest.Podman([]string{"inspect", "test_name", "--format={{.NetworkSettings.IPAddress}}"})
		IPAfter.WaitWithDefaultTimeout()
		Expect(IPAfter).Should(Exit(0))

		MACAfter := podmanTest.Podman([]string{"inspect", "test_name", "--format={{.NetworkSettings.MacAddress}}"})
		MACAfter.WaitWithDefaultTimeout()
		Expect(MACAfter).Should(Exit(0))

		// Check that IP address did not change between checkpointing and restoring
		Expect(IPBefore.OutputToString()).To(Equal(IPAfter.OutputToString()))

		// Check that MAC address did not change between checkpointing and restoring
		Expect(MACBefore.OutputToString()).To(Equal(MACAfter.OutputToString()))

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	// This test does the same steps which are necessary for migrating
	// a container from one host to another
	It("podman checkpoint container with export (migration)", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
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

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	// This test does the same steps which are necessary for migrating
	// a container from one host to another
	It("podman checkpoint container with export and different compression algorithms", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar"

		// Checkpoint with the default algorithm
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the zstd algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "--compress", "zstd"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the none algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "none"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the gzip algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "gzip"})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Checkpoint with the non-existing algorithm
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName, "-c", "non-existing"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result = podmanTest.Podman([]string{"rm", "--time", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
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
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "rm /etc/motd"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"diff", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("C /etc"))
		Expect(result.OutputToString()).To(ContainSubstring("A /test.output"))
		Expect(result.OutputToString()).To(ContainSubstring("D /etc/motd"))
		Expect(result.OutputToStringArray()).To(HaveLen(3))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		result = podmanTest.Podman([]string{"diff", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("C /etc"))
		Expect(result.OutputToString()).To(ContainSubstring("A /test.output"))
		Expect(result.OutputToString()).To(ContainSubstring("D /etc/motd"))
		Expect(result.OutputToStringArray()).To(HaveLen(3))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during restore", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "--ignore-rootfs", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(1))
		Expect(result.ErrorToString()).To(ContainSubstring("cat: can't open '/test.output': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})
	It("podman checkpoint and restore container with root file-system changes using --ignore-rootfs during checkpoint", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Change the container's root file-system
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", "--ignore-rootfs", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the changes to the container's root file-system
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(1))
		Expect(result.ErrorToString()).To(ContainSubstring("cat: can't open '/test.output': No such file or directory"))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and run exec in restored container", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Checkpoint the container
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore the container
		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Exec in the container
		result = podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"exec", cid, "cat", "/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint a container started with --rm", func() {
		// Start the container
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		cid := session.OutputToString()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Checkpoint the container - this should fail as it was started with --rm
		result := podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		Expect(result.ErrorToString()).To(ContainSubstring("cannot checkpoint containers that have been started with '--rm'"))

		// Checkpointing with --export should still work
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint a container with volumes", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/basicalpine/Containerfile.volume", "-t", "test-cr-volume",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Start the container
		localRunString := getRunString([]string{
			"--rm",
			"-v", "/volume1",
			"-v", "my-test-vol:/volume2",
			"test-cr-volume",
			"top",
		})
		session = podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		cid := session.OutputToString()

		// Add file in volume0
		result := podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume0/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Add file in volume1
		result = podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume1/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Add file in volume2
		result = podmanTest.Podman([]string{
			"exec", cid, "/bin/sh", "-c", "echo " + cid + " > /volume2/test.output",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		checkpointFileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		// Checkpoint the container
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container should fail because named volume still exists
		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		Expect(result.ErrorToString()).To(ContainSubstring(
			"volume with name my-test-vol already exists. Use --ignore-volumes to not restore content of volumes",
		))

		// Remove named volume
		session = podmanTest.Podman([]string{"volume", "rm", "my-test-vol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Restoring container
		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Validate volume0 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume0/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Validate volume1 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume1/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Validate volume2 content
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/volume2/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(cid))

		// Remove exported checkpoint
		os.Remove(checkpointFileName)
	})

	It("podman checkpoint container with --pre-checkpoint", func() {
		if !criu.MemTrack() {
			Skip("system (architecture/kernel/CRIU) does not support memory tracking")
		}
		if !strings.Contains(podmanTest.OCIRuntime, "runc") {
			Skip("Test only works on runc 1.0-rc3 or higher.")
		}
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "checkpoint", "-P", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "checkpoint", "--with-previous", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
	})

	It("podman checkpoint container with --pre-checkpoint and export (migration)", func() {
		SkipIfRemote("--import-previous is not yet supported on the remote client")
		if !criu.MemTrack() {
			Skip("system (architecture/kernel/CRIU) does not support memory tracking")
		}
		if !strings.Contains(podmanTest.OCIRuntime, "runc") {
			Skip("Test only works on runc 1.0-rc3 or higher.")
		}
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		preCheckpointFileName := "/tmp/pre-checkpoint-" + cid + ".tar.gz"
		checkpointFileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{"container", "checkpoint", "-P", "-e", preCheckpointFileName, cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"container", "checkpoint", "--with-previous", "-e", checkpointFileName, cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName, "--import-previous", preCheckpointFileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		os.Remove(checkpointFileName)
		os.Remove(preCheckpointFileName)
	})

	It("podman checkpoint and restore container with different port mappings", func() {
		randomPort, err := utils.GetRandomPort()
		Expect(err).ShouldNot(HaveOccurred())
		localRunString := getRunString([]string{"-p", fmt.Sprintf("%d:6379", randomPort), "--rm", redis})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		if !WaitContainerReady(podmanTest, cid, "Ready to accept connections", 20, 1) {
			Fail("Container failed to get ready")
		}

		fmt.Fprintf(os.Stderr, "Trying to connect to redis server at localhost:%d", randomPort)
		// Open a network connection to the redis server via initial port mapping
		conn, err := net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", randomPort), time.Duration(3)*time.Second)
		Expect(err).ShouldNot(HaveOccurred())
		conn.Close()

		// Checkpoint the container
		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Restore container with different port mapping
		newRandomPort, err := utils.GetRandomPort()
		Expect(err).ShouldNot(HaveOccurred())
		result = podmanTest.Podman([]string{"container", "restore", "-p", fmt.Sprintf("%d:6379", newRandomPort), "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Open a network connection to the redis server via initial port mapping
		// This should fail
		_, err = net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", randomPort), time.Duration(3)*time.Second)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).To(ContainSubstring("connection refused"))
		// Open a network connection to the redis server via new port mapping
		fmt.Fprintf(os.Stderr, "Trying to reconnect to redis server at localhost:%d", newRandomPort)
		conn, err = net.DialTimeout("tcp4", fmt.Sprintf("localhost:%d", newRandomPort), time.Duration(3)*time.Second)
		Expect(err).ShouldNot(HaveOccurred())
		conn.Close()

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	namespaceCombination := []string{
		"ipc,net,uts,pid",
		"ipc,net,uts",
		"ipc,net",
		"net,uts,pid",
		"net,uts",
		"uts,pid",
	}
	for _, share := range namespaceCombination {
		testName := fmt.Sprintf(
			"podman checkpoint and restore container out of and into pod (%s)",
			share,
		)

		share := share // copy into local scope, for use inside function

		It(testName, func() {
			if !criu.CheckForCriu(criu.PodCriuVersion) {
				Skip("CRIU is missing or too old.")
			}
			if !crutils.CRRuntimeSupportsPodCheckpointRestore(podmanTest.OCIRuntime) {
				Skip("runtime does not support pod restore: " + podmanTest.OCIRuntime)
			}
			// Create a pod
			session := podmanTest.Podman([]string{
				"pod",
				"create",
				"--share",
				share,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))
			podID := session.OutputToString()

			session = podmanTest.Podman([]string{
				"run",
				"-d",
				"--rm",
				"--pod",
				podID,
				ALPINE,
				"top",
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))
			cid := session.OutputToString()

			fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

			// Checkpoint the container
			result := podmanTest.Podman([]string{
				"container",
				"checkpoint",
				"-e",
				fileName,
				cid,
			})
			result.WaitWithDefaultTimeout()

			// As the container has been started with '--rm' it will be completely
			// cleaned up after checkpointing.
			Expect(result).To(Exit(0))
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
			Expect(podmanTest.NumberOfContainers()).To(Equal(1))

			// Remove the pod and create a new pod
			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				podID,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(Exit(0))

			// First create a pod with different shared namespaces.
			// Restore should fail

			wrongShare := share[:strings.LastIndex(share, ",")]

			session = podmanTest.Podman([]string{
				"pod",
				"create",
				"--share",
				wrongShare,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))
			podID = session.OutputToString()

			// Restore container with different port mapping
			result = podmanTest.Podman([]string{
				"container",
				"restore",
				"--pod",
				podID,
				"-i",
				fileName,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(Exit(125))
			Expect(result.ErrorToString()).To(ContainSubstring("does not share the"))

			// Remove the pod and create a new pod
			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				podID,
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(Exit(0))

			session = podmanTest.Podman([]string{
				"pod",
				"create",
				"--share",
				share,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))
			podID = session.OutputToString()

			// Restore container with different port mapping
			result = podmanTest.Podman([]string{
				"container",
				"restore",
				"--pod",
				podID,
				"-i",
				fileName,
			})
			result.WaitWithDefaultTimeout()

			Expect(result).To(Exit(0))
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
			Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

			result = podmanTest.Podman([]string{
				"rm",
				"-f",
				result.OutputToString(),
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(Exit(0))
			Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
			Expect(podmanTest.NumberOfContainers()).To(Equal(1))

			result = podmanTest.Podman([]string{
				"pod",
				"rm",
				"-fa",
			})
			result.WaitWithDefaultTimeout()
			Expect(result).To(Exit(0))

			// Remove exported checkpoint
			os.Remove(fileName)
		})
	}

	It("podman checkpoint container with export (migration) and --ipc host", func() {
		localRunString := getRunString([]string{"--rm", "--ipc", "host", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", fileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", fileName})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint container with export and statistics", func() {
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()
		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// Extract checkpoint archive
		destinationDirectory, err := CreateTempDirInTempDir()
		Expect(err).ShouldNot(HaveOccurred())

		tarsession := SystemExec(
			"tar",
			[]string{
				"xf",
				fileName,
				"-C",
				destinationDirectory,
			},
		)
		Expect(tarsession).Should(Exit(0))

		_, err = os.Stat(filepath.Join(destinationDirectory, stats.StatsDump))
		Expect(err).ShouldNot(HaveOccurred())

		Expect(os.RemoveAll(destinationDirectory)).To(BeNil())

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and restore containers with --print-stats", func() {
		session1 := podmanTest.Podman(getRunString([]string{redis}))
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		session2 := podmanTest.Podman(getRunString([]string{redis, "top"}))
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			"-a",
			"--print-stats",
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		type checkpointStatistics struct {
			PodmanDuration      int64                        `json:"podman_checkpoint_duration"`
			ContainerStatistics []*entities.CheckpointReport `json:"container_statistics"`
		}

		cS := new(checkpointStatistics)
		err := json.Unmarshal([]byte(result.OutputToString()), cS)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(cS.ContainerStatistics).To(HaveLen(2))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[0].RuntimeDuration))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[1].RuntimeDuration))
		Expect(cS.ContainerStatistics[0].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[0].CRIUStatistics.FrozenTime),
		)
		Expect(cS.ContainerStatistics[1].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[1].CRIUStatistics.FrozenTime),
		)

		ps := podmanTest.Podman([]string{
			"ps",
			"-q",
			"--no-trunc",
		})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session1.OutputToString())))
		Expect(ps.OutputToString()).To(Not(ContainSubstring(session2.OutputToString())))

		result = podmanTest.Podman([]string{
			"container",
			"restore",
			"-a",
			"--print-stats",
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))
		Expect(podmanTest.GetContainerStatus()).To(Not(ContainSubstring("Exited")))

		type restoreStatistics struct {
			PodmanDuration      int64                     `json:"podman_restore_duration"`
			ContainerStatistics []*entities.RestoreReport `json:"container_statistics"`
		}

		rS := new(restoreStatistics)
		err = json.Unmarshal([]byte(result.OutputToString()), rS)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(cS.ContainerStatistics).To(HaveLen(2))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[0].RuntimeDuration))
		Expect(cS.PodmanDuration).To(BeNumerically(">", cS.ContainerStatistics[1].RuntimeDuration))
		Expect(cS.ContainerStatistics[0].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[0].CRIUStatistics.RestoreTime),
		)
		Expect(cS.ContainerStatistics[1].RuntimeDuration).To(
			BeNumerically(">", cS.ContainerStatistics[1].CRIUStatistics.RestoreTime),
		)

		result = podmanTest.Podman([]string{
			"rm",
			"-t",
			"0",
			"-fa",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint and restore container with --file-locks", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "runc") {
			// TODO: Enable test for crun when this feature has been released
			// https://github.com/containers/crun/pull/783
			Skip("FIXME: requires crun >= 1.4")
		}
		localRunString := getRunString([]string{"--name", "test_name", ALPINE, "flock", "test.lock", "sleep", "100"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Checkpoint is expected to fail without --file-locks
		result := podmanTest.Podman([]string{"container", "checkpoint", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("failed: exit status 1"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		// Checkpoint is expected to succeed with --file-locks
		result = podmanTest.Podman([]string{"container", "checkpoint", "--file-locks", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", "--file-locks", "test_name"})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-f", "test_name"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman checkpoint container with export and verify runtime", func() {
		SkipIfRemote("podman-remote does not support --runtime flag")
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		runtime := session.OutputToString()

		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// The restored container should have the same runtime as the original container
		result = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(runtime))

		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint container with export and try to change the runtime", func() {
		SkipIfRemote("podman-remote does not support --runtime flag")
		// This test will only run if runc and crun both exist
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test requires crun and runc")
		}
		cmd := exec.Command("runc")
		if err := cmd.Start(); err != nil {
			Skip("Test requires crun and runc")
		}
		if err := cmd.Wait(); err != nil {
			Skip("Test requires crun and runc")
		}
		localRunString := getRunString([]string{
			"--rm",
			ALPINE,
			"top",
		})
		// Let's start a container with runc and try to restore it with crun (expected to fail)
		localRunString = append(
			[]string{
				"--runtime",
				"runc",
			},
			localRunString...,
		)
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		runtime := session.OutputToString()

		fileName := "/tmp/checkpoint-" + cid + ".tar.gz"

		result := podmanTest.Podman([]string{
			"container",
			"checkpoint",
			cid, "-e",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		// This should fail as the container was checkpointed with runc
		result = podmanTest.Podman([]string{
			"--runtime",
			"crun",
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(
			ContainSubstring("and cannot be restored with runtime"),
		)

		result = podmanTest.Podman([]string{
			"--runtime",
			"runc",
			"container",
			"restore",
			"-i",
			fileName,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{
			"inspect",
			"--format",
			"{{.OCIRuntime}}",
			cid,
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(runtime))

		result = podmanTest.Podman([]string{
			"--runtime",
			"runc",
			"rm",
			"-fa",
		})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		// Remove exported checkpoint
		os.Remove(fileName)
	})

	It("podman checkpoint and restore dev/shm content with --export and --import", func() {
		localRunString := getRunString([]string{"--rm", ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		// Add test file in dev/shm
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		runtime := session.OutputToString()

		checkpointFileName := "/tmp/checkpoint-" + cid + ".tar.gz"
		result = podmanTest.Podman([]string{"container", "checkpoint", cid, "-e", checkpointFileName})
		result.WaitWithDefaultTimeout()

		// As the container has been started with '--rm' it will be completely
		// cleaned up after checkpointing.
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))

		result = podmanTest.Podman([]string{"container", "restore", "-i", checkpointFileName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// The restored container should have the same runtime as the original container
		result = podmanTest.Podman([]string{"inspect", "--format", "{{.OCIRuntime}}", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(runtime))

		// Verify the test file content in dev/shm
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		// Remove exported checkpoint
		os.Remove(checkpointFileName)
	})

	It("podman checkpoint and restore dev/shm content", func() {
		localRunString := getRunString([]string{ALPINE, "top"})
		session := podmanTest.Podman(localRunString)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		cid := session.OutputToString()

		// Add test file in dev/shm
		result := podmanTest.Podman([]string{"exec", cid, "/bin/sh", "-c", "echo test" + cid + "test > /dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"container", "checkpoint", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Exited"))

		result = podmanTest.Podman([]string{"container", "restore", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring("Up"))

		// Verify the test file content in dev/shm
		result = podmanTest.Podman([]string{"exec", cid, "cat", "/dev/shm/test.output"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("test" + cid + "test"))

		result = podmanTest.Podman([]string{"rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
