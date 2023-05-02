package integration

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod start", func() {
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

	It("podman pod start bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod start single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "start", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod start single pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pod start multiple pods", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec2).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("multiple pods in conflict", func() {
		podName := []string{"Pod_" + RandomString(10), "Pod_" + RandomString(10)}

		pod, _, podid1 := podmanTest.CreatePod(map[string][]string{
			"--infra":   {"true"},
			"--name":    {podName[0]},
			"--publish": {"127.0.0.1:8083:80"},
		})
		Expect(pod).To(Exit(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName[0], ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		pod, _, podid2 := podmanTest.CreatePod(map[string][]string{
			"--infra":   {"true"},
			"--name":    {podName[1]},
			"--publish": {"127.0.0.1:8083:80"},
		})
		Expect(pod).To(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", podName[1], ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(125))
	})

	It("podman pod start all pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start latest pod", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		podid := "--latest"
		if IsRemote() {
			podid = "foobar100"
		}
		session = podmanTest.Podman([]string{"pod", "start", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod start multiple pods with bogus", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", podid, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman pod start single pod via --pod-id-file", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "podID"
		defer os.RemoveAll(tmpDir)

		podName := "rudolph"

		// Create a pod with --pod-id-file.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Create container inside the pod.
		session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2)) // infra+top
	})

	It("podman pod start multiple pods via --pod-id-file", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		podIDFiles := []string{}
		for _, i := range "0123456789" {
			tmpFile := tmpDir + "cid" + string(i)
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, tmpFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)
	})

	It("podman pod create --infra-conmon-pod create + start", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "podID"
		defer os.RemoveAll(tmpDir)

		podName := "rudolph"
		// Create a pod with --infra-conmon-pid.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--infra-conmon-pidfile", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "start", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1)) // infra

		readFirstLine := func(path string) string {
			content, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			return strings.Split(string(content), "\n")[0]
		}

		// Read the infra-conmon-pidfile and perform some sanity checks
		// on the pid.
		infraConmonPID := readFirstLine(tmpFile)
		_, err = strconv.Atoi(infraConmonPID) // Make sure it's a proper integer
		Expect(err).ToNot(HaveOccurred())

		cmdline := readFirstLine(fmt.Sprintf("/proc/%s/cmdline", infraConmonPID))
		Expect(cmdline).To(ContainSubstring("/conmon"))
	})

})
