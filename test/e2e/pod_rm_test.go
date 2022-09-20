package integration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pod rm", func() {
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
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman pod rm empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		// Also check that we don't leak cgroups
		err := filepath.WalkDir("/sys/fs/cgroup", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				Expect(err).To(BeNil())
			}
			if strings.Contains(d.Name(), podid) {
				return fmt.Errorf("leaking cgroup path %s", path)
			}
			return nil
		})
		Expect(err).To(BeNil())
	})

	It("podman pod rm latest pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"pod2"}})
		Expect(ec2).To(Equal(0))

		latest := "--latest"
		if IsRemote() {
			latest = "pod2"
		}
		result := podmanTest.Podman([]string{"pod", "rm", latest})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(podid))
		Expect(result.OutputToString()).To(Not(ContainSubstring(podid2)))
	})

	It("podman pod rm removes a pod with a container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.RunLsContainerInPod("", podid)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})

	It("podman pod rm -f does remove a running container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})

	It("podman pod rm -a doesn't remove a running container", func() {
		fmt.Printf("To start, there are %d pods\n", podmanTest.NumberOfPods())
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))
		fmt.Printf("Started %d pods\n", podmanTest.NumberOfPods())

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podmanTest.WaitForContainer()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		fmt.Printf("Started container running in one pod")

		numPods := podmanTest.NumberOfPods()
		Expect(numPods).To(Equal(2))
		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		fmt.Printf("Current %d pod(s):\n%s\n", numPods, ps.OutputToString())

		fmt.Printf("Removing all empty pods\n")
		result := podmanTest.Podman([]string{"pod", "rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		foundExpectedError, _ := result.ErrorGrepString("cannot be removed")
		Expect(foundExpectedError).To(Equal(true))

		numPods = podmanTest.NumberOfPods()
		ps = podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		fmt.Printf("Final %d pod(s):\n%s\n", numPods, ps.OutputToString())
		Expect(numPods).To(Equal(1))
		// Confirm top container still running inside remaining pod
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod rm -fa removes everything", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "--pod", podid1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec, _ = podmanTest.RunLsContainerInPod("", podid2)
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		result = podmanTest.Podman([]string{"ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())

		// one pod should have been deleted
		result = podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(BeEmpty())
	})

	It("podman rm bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman rm bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		session = podmanTest.Podman([]string{"pod", "rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman rm --ignore bogus pod and a running pod", func() {

		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pod start/remove single pod via --pod-id-file", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).To(BeNil())
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

		session = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "--pod-id-file", tmpFile, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod start/remove multiple pods via --pod-id-file", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).To(BeNil())
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

		cmd = []string{"pod", "rm", "--time=0", "--force"}
		cmd = append(cmd, podIDFiles...)
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod rm with exited containers", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "--pod", podid, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--pod", podid, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"pod", "rm", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman pod rm pod with infra container and running container", func() {
		podName := "testPod"
		ctrName := "testCtr"

		ctrAndPod := podmanTest.Podman([]string{"run", "-d", "--pod", fmt.Sprintf("new:%s", podName), "--name", ctrName, ALPINE, "top"})
		ctrAndPod.WaitWithDefaultTimeout()
		Expect(ctrAndPod).Should(Exit(0))

		removePod := podmanTest.Podman([]string{"pod", "rm", "-a"})
		removePod.WaitWithDefaultTimeout()
		Expect(removePod).Should(Not(Exit(0)))

		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToString()).To(ContainSubstring(podName))

		removePodForce := podmanTest.Podman([]string{"pod", "rm", "-af"})
		removePodForce.WaitWithDefaultTimeout()
		Expect(removePodForce).Should(Exit(0))

		ps2 := podmanTest.Podman([]string{"pod", "ps"})
		ps2.WaitWithDefaultTimeout()
		Expect(ps2).Should(Exit(0))
		Expect(ps2.OutputToString()).To(Not(ContainSubstring(podName)))
	})
})
