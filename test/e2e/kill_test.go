package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman kill", func() {
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

	It("podman kill bogus container", func() {
		session := podmanTest.Podman([]string{"kill", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman container kill a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "kill", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman container kill a running container by short id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "kill", cid[:5]})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(Equal(cid[:5]))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id with TERM", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", "-s", "9", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id with a bogus signal", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", "-s", "foobar", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman kill latest container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		cid := "-l"
		if IsRemote() {
			cid = "test1"
		}
		result := podmanTest.Podman([]string{"kill", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill paused container", func() {
		SkipIfRootlessCgroupsV1("pause is not supported for cgroupv1 rootless")
		ctrName := "testctr"
		session := podmanTest.RunTopContainer(ctrName)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		pause := podmanTest.Podman([]string{"pause", ctrName})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(Exit(0))

		kill := podmanTest.Podman([]string{"kill", ctrName})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "-f", "{{.State.Status}}", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Or(Equal("stopped"), Equal("exited")))
	})

	It("podman kill --cidfile", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "cid"
		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"run", "-dt", "--cidfile", tmpFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToStringArray()[0]

		kill := podmanTest.Podman([]string{"kill", "--cidfile", tmpFile})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(Exit(0))

		wait := podmanTest.Podman([]string{"wait", "--condition", "exited", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))
	})

	It("podman kill multiple --cidfile", func() {
		tmpDir1, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile1 := tmpDir1 + "cid"
		defer os.RemoveAll(tmpDir1)

		tmpDir2, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile2 := tmpDir2 + "cid"
		defer os.RemoveAll(tmpDir2)

		session := podmanTest.Podman([]string{"run", "-dt", "--cidfile", tmpFile1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToStringArray()[0]

		session2 := podmanTest.Podman([]string{"run", "-dt", "--cidfile", tmpFile2, ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		cid2 := session2.OutputToStringArray()[0]

		kill := podmanTest.Podman([]string{"kill", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(Exit(0))

		wait := podmanTest.Podman([]string{"wait", "--condition", "exited", cid1})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))
		wait = podmanTest.Podman([]string{"wait", "--condition", "exited", cid2})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))
	})

	It("podman kill --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		session = podmanTest.Podman([]string{"kill", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
