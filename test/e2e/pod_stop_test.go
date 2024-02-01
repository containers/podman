package integration

import (
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod stop", func() {

	It("podman pod stop bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stop", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod stop --ignore bogus pod", func() {

		session := podmanTest.Podman([]string{"pod", "stop", "--ignore", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stop bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman stop --ignore bogus pod and a running pod", func() {

		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "-t", "-1", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod stop single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "stop", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod stop single pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop multiple pods", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec2).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop all pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop latest pod", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		podid := "--latest"
		if IsRemote() {
			podid = "foobar100"
		}
		session = podmanTest.Podman([]string{"pod", "stop", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod stop multiple pods with bogus", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", podid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod start/stop single pod via --pod-id-file", func() {
		tmpDir := GinkgoT().TempDir()
		tmpFile := tmpDir + "podID"

		podName := "rudolph"

		// Create a pod with --pod-id-file.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Create container inside the pod.
		session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2)) // infra+top

		session = podmanTest.Podman([]string{"pod", "stop", "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod start/stop multiple pods via --pod-id-file", func() {
		tmpDir := GinkgoT().TempDir()

		podIDFiles := []string{}
		for _, i := range "0123456789" {
			tmpFile := tmpDir + "cid" + string(i)
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, tmpFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)

		cmd = []string{"pod", "stop"}
		cmd = append(cmd, podIDFiles...)
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
