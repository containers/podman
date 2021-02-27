package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.SeedImages()
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
		Expect(result.ExitCode()).To(Equal(0))

		// Also check that we don't leak cgroups
		err := filepath.Walk("/sys/fs/cgroup", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				Expect(err).To(BeNil())
			}
			if strings.Contains(info.Name(), podid) {
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
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
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
		Expect(result.ExitCode()).To(Equal(0))

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman pod rm -f does remove a running container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-f", podid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

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
		Expect(session.ExitCode()).To(Equal(0))
		podmanTest.WaitForContainer()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		fmt.Printf("Started container running in one pod")

		num_pods := podmanTest.NumberOfPods()
		Expect(num_pods).To(Equal(2))
		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		fmt.Printf("Current %d pod(s):\n%s\n", num_pods, ps.OutputToString())

		fmt.Printf("Removing all empty pods\n")
		result := podmanTest.Podman([]string{"pod", "rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		foundExpectedError, _ := result.ErrorGrepString("cannot be removed")
		Expect(foundExpectedError).To(Equal(true))

		num_pods = podmanTest.NumberOfPods()
		ps = podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		fmt.Printf("Final %d pod(s):\n%s\n", num_pods, ps.OutputToString())
		Expect(num_pods).To(Equal(1))
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
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", podid1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainerInPod("", podid2)
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "rm", "-fa"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

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
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("podman rm bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))

		session = podmanTest.Podman([]string{"pod", "rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("podman rm --ignore bogus pod and a running pod", func() {

		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "rm", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pod start/remove single pod via --pod-id-file", func() {
		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		tmpFile := tmpDir + "podID"
		defer os.RemoveAll(tmpDir)

		podName := "rudolph"

		// Create a pod with --pod-id-file.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Create container inside the pod.
		session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "--pod-id-file", tmpFile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2)) // infra+top

		session = podmanTest.Podman([]string{"pod", "rm", "--pod-id-file", tmpFile, "--force"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod start/remove multiple pods via --pod-id-file", func() {
		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		defer os.RemoveAll(tmpDir)

		podIDFiles := []string{}
		for _, i := range "0123456789" {
			tmpFile := tmpDir + "cid" + string(i)
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", tmpFile})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, tmpFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)

		cmd = []string{"pod", "rm", "--force"}
		cmd = append(cmd, podIDFiles...)
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
