package integration

import (
	"io/ioutil"
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman stop bogus container", func() {
		session := podmanTest.Podman([]string{"stop", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman stop --ignore bogus container", func() {
		SkipIfRemote()

		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "--ignore", "foobar", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"stop", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman container stop by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"container", "stop", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop stopped container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"stop", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		session3 := podmanTest.Podman([]string{"stop", "test1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3.ExitCode()).To(Equal(0))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers -t", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToString()

		session = podmanTest.RunTopContainer("test2")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid2 := session.OutputToString()

		session = podmanTest.RunTopContainer("test3")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid3 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(output).To(ContainSubstring(cid3))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --time", func() {
		session := podmanTest.RunTopContainer("test4")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "--time", "1", "test4"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop container --timeout", func() {
		session := podmanTest.RunTopContainer("test5")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToString()

		session = podmanTest.Podman([]string{"stop", "--timeout", "1", "test5"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(ContainSubstring(cid1))

		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop latest containers", func() {
		SkipIfRemote()
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"stop", "-l", "-t", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one stopped", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session2 := podmanTest.RunTopContainer("test2")
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		session3 := podmanTest.Podman([]string{"stop", "-l", "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3.ExitCode()).To(Equal(0))
		session4 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session4.WaitWithDefaultTimeout()
		Expect(session4.ExitCode()).To(Equal(0))
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop all containers with one created", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session2 := podmanTest.Podman([]string{"create", ALPINE, "/bin/sh"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		session3 := podmanTest.Podman([]string{"stop", "-a", "-t", "1"})
		session3.WaitWithDefaultTimeout()
		Expect(session3.ExitCode()).To(Equal(0))
		finalCtrs := podmanTest.Podman([]string{"ps", "-q"})
		finalCtrs.WaitWithDefaultTimeout()
		Expect(finalCtrs.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(finalCtrs.OutputToString())).To(Equal(""))
	})

	It("podman stop --cidfile", func() {
		SkipIfRemote()

		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		tmpFile := tmpDir + "cid"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToStringArray()[0]

		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"stop", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
	})

	It("podman stop multiple --cidfile", func() {
		SkipIfRemote()

		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		tmpFile1 := tmpDir + "cid-1"
		tmpFile2 := tmpDir + "cid-2"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"run", "--cidfile", tmpFile1, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"run", "--cidfile", tmpFile2, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"stop", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("podman stop invalid --latest and --cidfile and --all", func() {
		SkipIfRemote()

		result := podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"stop", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"stop", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})
})
