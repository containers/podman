package integration

import (
	"io/ioutil"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman rm", func() {
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

	It("podman rm stopped container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman rm refuse to remove a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(2))
	})

	It("podman rm created container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman container rm created container", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman rm running container with -f", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman rm all containers", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman rm all containers with one running and short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", "-af"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman rm the latest container", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

	})

	It("podman rm --cidfile", func() {
		SkipIfRemote()

		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		tmpFile := tmpDir + "cid"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"rm", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm multiple --cidfile", func() {
		SkipIfRemote()

		tmpDir, err := ioutil.TempDir("", "")
		Expect(err).To(BeNil())
		tmpFile1 := tmpDir + "cid-1"
		tmpFile2 := tmpDir + "cid-2"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", "--cidfile", tmpFile2, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"rm", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm invalid --latest and --cidfile and --all", func() {
		SkipIfRemote()

		result := podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))

		result = podmanTest.Podman([]string{"rm", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman rm bogus container", func() {
		session := podmanTest.Podman([]string{"rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("podman rm bogus container and a running container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))

		session = podmanTest.Podman([]string{"rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman rm --ignore bogus container and a running container", func() {
		SkipIfRemote()

		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rm", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
