package integration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/podman/v4/libpod/define"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman init containers", func() {
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

	It("podman create init container without --pod should fail", func() {
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman create init container with bad init type should fail", func() {
		session := podmanTest.Podman([]string{"create", "--init-ctr", "unknown", "--pod", "new:foobar", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman init containers should not degrade pod status", func() {
		// create a pod
		topPod := podmanTest.Podman([]string{"create", "-t", "--pod", "new:foobar", ALPINE, "top"})
		topPod.WaitWithDefaultTimeout()
		Expect(topPod).Should(Exit(0))
		// add an init container
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "foobar", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// start a pod
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		data := inspect.InspectPodToJSON()
		Expect(data.State).To(Equal(define.PodStateRunning))
	})

	It("podman create init container should fail in running pod", func() {
		// create a running pod
		topPod := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobar", ALPINE, "top"})
		topPod.WaitWithDefaultTimeout()
		Expect(topPod).Should(Exit(0))
		// adding init-ctr to running pod should fail
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "foobar", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman make sure init container runs before pod containers", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))
		content := RandomString(16)
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "new:foobar", ALPINE, "bin/sh", "-c", fmt.Sprintf("echo %s > %s", content, filename)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(Exit(0))
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))
		checkLog := podmanTest.Podman([]string{"exec", "-it", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		Expect(checkLog).Should(Exit(0))
		Expect(checkLog.OutputToString()).To(Equal(content))
	})

	It("podman make sure once container is removed", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))
		content := RandomString(16)
		session := podmanTest.Podman([]string{"create", "--init-ctr", "once", "--pod", "new:foobar", ALPINE, "bin/sh", "-c", fmt.Sprintf("echo %s > %s", content, filename)})
		session.WaitWithDefaultTimeout()
		initContainerID := session.OutputToString()
		Expect(session).Should(Exit(0))
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(Exit(0))
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))
		check := podmanTest.Podman([]string{"container", "exists", initContainerID})
		check.WaitWithDefaultTimeout()
		// Container was rm'd
		//Expect(check).Should(Exit(1))
		Expect(check.ExitCode()).To(Equal(1), "I dont understand why the other way does not work")
		// Lets double check with a stop and start
		stopPod := podmanTest.Podman([]string{"pod", "stop", "foobar"})
		stopPod.WaitWithDefaultTimeout()
		Expect(stopPod).Should(Exit(0))
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(Exit(0))

		// Because no init was run, the file should not even exist
		doubleCheck := podmanTest.Podman([]string{"exec", "-it", verify.OutputToString(), "cat", filename})
		doubleCheck.WaitWithDefaultTimeout()
		Expect(doubleCheck).Should(Exit(1))

	})

	It("podman ensure always init containers always run", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))

		// Write the date to a file
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "new:foobar", fedoraMinimal, "bin/sh", "-c", "date +%T.%N > " + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(Exit(0))
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))

		// capture the date written
		checkLog := podmanTest.Podman([]string{"exec", "-it", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		firstResult := checkLog.OutputToString()
		Expect(checkLog).Should(Exit(0))

		// Stop and start the pod
		stopPod := podmanTest.Podman([]string{"pod", "stop", "foobar"})
		stopPod.WaitWithDefaultTimeout()
		Expect(stopPod).Should(Exit(0))
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(Exit(0))

		// Check the file again with exec
		secondCheckLog := podmanTest.Podman([]string{"exec", "-it", verify.OutputToString(), "cat", filename})
		secondCheckLog.WaitWithDefaultTimeout()
		Expect(secondCheckLog).Should(Exit(0))

		// Dates should not match
		Expect(firstResult).ToNot(Equal(secondCheckLog.OutputToString()))
	})

})
