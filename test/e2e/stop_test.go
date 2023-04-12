package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman stop", func() {
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

	It("podman stop bogus container", func() {
		session := podmanTest.Podman([]string{"stop", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman stop --ignore bogus container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "--ignore", "foobar", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stop", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop single container by short id", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		shortID := cid[0:10]

		session = podmanTest.Podman([]string{"stop", shortID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(shortID))
	})

	It("podman stop container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman container stop by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"container", "stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop stopped container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"stop", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		session3 := podmanTest.Podman([]string{"stop", "test1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers -t", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid2 := session.OutputToString()

		session = podmanTest.RunTopContainer("test3")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid3 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(output).To(ContainSubstring(cid3))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --time", func() {
		session := podmanTest.RunTopContainer("test4")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"stop", "--time", "0", "test4"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("test4"))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --timeout", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test5", ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stop", "--timeout", "0", "test5"})
		// Without timeout container stops in 10 seconds
		// If not stopped in 5 seconds, then --timeout did not work
		session.Wait(5)
		Expect(session).Should(Exit(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("test5"))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --timeout Warning", func() {
		SkipIfRemote("warning will happen only on server side")
		session := podmanTest.Podman([]string{"run", "-d", "--name", "test5", ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"stop", "--timeout", "1", "test5"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		warning := session.ErrorToString()
		Expect(warning).To(ContainSubstring("StopSignal SIGTERM failed to stop container test5 in 1 seconds, resorting to SIGKILL"))
	})

	It("podman stop latest containers", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-l", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(cid))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one stopped", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session2 := podmanTest.RunTopContainer("test2")
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		cid := "-l"
		if IsRemote() {
			cid = "test2"
		}
		session3 := podmanTest.Podman([]string{"stop", cid, "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))
		session4 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session4.WaitWithDefaultTimeout()
		Expect(session4).Should(Exit(0))
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one created", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session2 := podmanTest.Podman([]string{"create", ALPINE, "/bin/sh"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		session3 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs).Should(Exit(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop should return silent success on stopping configured containers", func() {
		// following container is not created on OCI runtime
		// so we return success and assume that it is stopped
		session2 := podmanTest.Podman([]string{"create", "--name", "stopctr", ALPINE, "/bin/sh"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		session3 := podmanTest.Podman([]string{"stop", "stopctr"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(Exit(0))
	})

	It("podman stop --cidfile", func() {

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "cid"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"stop", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop multiple --cidfile", func() {

		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile1 := tmpDir + "cid-1"
		tmpFile2 := tmpDir + "cid-2"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"run", "--cidfile", tmpFile1, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"run", "--cidfile", tmpFile2, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"stop", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman stop invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("--latest flag n/a")

		result := podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))

		result = podmanTest.Podman([]string{"stop", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman stop --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		session = podmanTest.Podman([]string{"stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop --ignore", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.Podman([]string{"stop", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))

		session = podmanTest.Podman([]string{"stop", "--ignore", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman stop --filter", func() {
		session1 := podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid1 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid2 := session1.OutputToString()

		session1 = podmanTest.Podman([]string{"container", "create", ALPINE})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.Podman([]string{"start", "--all"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))

		session1 = podmanTest.Podman([]string{"stop", cid1, "-f", "status=running"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(125))

		session1 = podmanTest.Podman([]string{"stop", "-a", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(HaveLen(0))

		session1 = podmanTest.Podman([]string{"stop", "-a", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"stop", "-f", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})
})
