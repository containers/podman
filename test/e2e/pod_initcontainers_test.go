package integration

import (
	"fmt"
	"path/filepath"

	"github.com/containers/podman/v4/libpod/define"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman init containers", func() {

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
		Expect(topPod).Should(ExitCleanly())
		// add an init container
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "foobar", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// start a pod
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		data := inspect.InspectPodToJSON()
		Expect(data).To(HaveField("State", define.PodStateRunning))
	})

	It("podman create init container should fail in running pod", func() {
		// create a running pod
		topPod := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobar", ALPINE, "top"})
		topPod.WaitWithDefaultTimeout()
		Expect(topPod).Should(ExitCleanly())
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
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())
		checkLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		Expect(checkLog).Should(ExitCleanly())
		Expect(checkLog.OutputToString()).To(Equal(content))
	})

	It("podman make sure once container is removed", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))
		content := RandomString(16)
		session := podmanTest.Podman([]string{"create", "--init-ctr", "once", "--pod", "new:foobar", ALPINE, "bin/sh", "-c", fmt.Sprintf("echo %s > %s", content, filename)})
		session.WaitWithDefaultTimeout()
		initContainerID := session.OutputToString()
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())
		check := podmanTest.Podman([]string{"container", "exists", initContainerID})
		check.WaitWithDefaultTimeout()
		// Container was rm'd
		// Expect(check).Should(Exit(1))
		Expect(check.ExitCode()).To(Equal(1), "I dont understand why the other way does not work")
		// Let's double check with a stop and start
		stopPod := podmanTest.Podman([]string{"pod", "stop", "foobar"})
		stopPod.WaitWithDefaultTimeout()
		Expect(stopPod).Should(ExitCleanly())
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(ExitCleanly())

		// Because no init was run, the file should not even exist
		doubleCheck := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		doubleCheck.WaitWithDefaultTimeout()
		Expect(doubleCheck).Should(Exit(1))

	})

	It("podman ensure always init containers always run", func() {
		filename := filepath.Join("/dev/shm", RandomString(12))

		// Write the date to a file
		session := podmanTest.Podman([]string{"create", "--init-ctr", "always", "--pod", "new:foobar", fedoraMinimal, "bin/sh", "-c", "date +%T.%N > " + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		verify := podmanTest.Podman([]string{"create", "--pod", "foobar", "-t", ALPINE, "top"})
		verify.WaitWithDefaultTimeout()
		Expect(verify).Should(ExitCleanly())
		start := podmanTest.Podman([]string{"pod", "start", "foobar"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		// capture the date written
		checkLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		checkLog.WaitWithDefaultTimeout()
		firstResult := checkLog.OutputToString()
		Expect(checkLog).Should(ExitCleanly())

		// Stop and start the pod
		stopPod := podmanTest.Podman([]string{"pod", "stop", "foobar"})
		stopPod.WaitWithDefaultTimeout()
		Expect(stopPod).Should(ExitCleanly())
		startPod := podmanTest.Podman([]string{"pod", "start", "foobar"})
		startPod.WaitWithDefaultTimeout()
		Expect(startPod).Should(ExitCleanly())

		// Check the file again with exec
		secondCheckLog := podmanTest.Podman([]string{"exec", verify.OutputToString(), "cat", filename})
		secondCheckLog.WaitWithDefaultTimeout()
		Expect(secondCheckLog).Should(ExitCleanly())

		// Dates should not match
		Expect(firstResult).ToNot(Equal(secondCheckLog.OutputToString()))
	})

})
