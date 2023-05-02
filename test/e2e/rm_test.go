package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman rm stopped container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman rm refuse to remove a running container", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(2))
		Expect(result.ErrorToString()).To(ContainSubstring("containers cannot be removed without force"))
	})

	It("podman rm created container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman container rm created container", func() {
		session := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "rm", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman rm running container with -f", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"rm", "-t", "0", "-f", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman rm all containers", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"rm", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman rm all containers with one running and short options", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"rm", "-af"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman rm the latest container", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		_, ec, cid := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))

		latest := "-l"
		if IsRemote() {
			latest = cid
		}
		result := podmanTest.Podman([]string{"rm", latest})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))
	})

	It("podman rm --cidfile", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile := tmpDir + "cid"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		result := podmanTest.Podman([]string{"rm", "--cidfile", tmpFile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm multiple --cidfile", func() {
		tmpDir, err := os.MkdirTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		tmpFile1 := tmpDir + "cid-1"
		tmpFile2 := tmpDir + "cid-2"

		defer os.RemoveAll(tmpDir)

		session := podmanTest.Podman([]string{"create", "--cidfile", tmpFile1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid1 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", "--cidfile", tmpFile2, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid2 := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		result := podmanTest.Podman([]string{"rm", "--cidfile", tmpFile1, "--cidfile", tmpFile2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		output := result.OutputToString()
		Expect(output).To(ContainSubstring(cid1))
		Expect(output).To(ContainSubstring(cid2))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm invalid --latest and --cidfile and --all", func() {
		SkipIfRemote("Verifying --latest flag")

		result := podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--cidfile", "foobar", "--all", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("--all, --latest, and --cidfile cannot be used together"))

		result = podmanTest.Podman([]string{"rm", "--latest", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
		Expect(result.ErrorToString()).To(ContainSubstring("--all and --latest cannot be used together"))
	})

	It("podman rm --all", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))

		session = podmanTest.Podman([]string{"rm", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm --ignore", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		cid := session.OutputToStringArray()[0]
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session = podmanTest.Podman([]string{"rm", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("\"bogus\" found: no such container"))
		if IsRemote() {
			Expect(session.OutputToString()).To(BeEquivalentTo(cid))
		}

		session = podmanTest.Podman([]string{"rm", "--ignore", "bogus", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		if !IsRemote() {
			Expect(session.OutputToString()).To(BeEquivalentTo(cid))
		}
		Expect(podmanTest.NumberOfContainers()).To(Equal(0))
	})

	It("podman rm bogus container", func() {
		session := podmanTest.Podman([]string{"rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("\"bogus\" found: no such container"))
	})

	It("podman rm bogus container and a running container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rm", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("\"bogus\" found: no such container"))

		session = podmanTest.Podman([]string{"rm", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("\"bogus\" found: no such container"))
	})

	It("podman rm --ignore bogus container and a running container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rm", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(2))
		Expect(session.ErrorToString()).To(ContainSubstring("containers cannot be removed without force"))

		session = podmanTest.Podman([]string{"rm", "-t", "0", "--force", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeEquivalentTo("test1"))
	})

	It("podman rm --filter", func() {
		session1 := podmanTest.RunTopContainer("test1")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid1 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("test2")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid2 := session1.OutputToString()

		session1 = podmanTest.RunTopContainer("test3")
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		cid3 := session1.OutputToString()
		shortCid3 := cid3[0:5]

		session1 = podmanTest.Podman([]string{"rm", cid1, "-f", "--filter", "status=running"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(125))
		Expect(session1.ErrorToString()).To(ContainSubstring("--filter takes no arguments"))

		session1 = podmanTest.Podman([]string{"rm", "-a", "-f", "--filter", fmt.Sprintf("id=%swrongid", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(HaveLen(0))

		session1 = podmanTest.Podman([]string{"rm", "-a", "-f", "--filter", fmt.Sprintf("id=%s", shortCid3)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid3))

		session1 = podmanTest.Podman([]string{"rm", "-f", "--filter", fmt.Sprintf("id=%s", cid2)})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(Exit(0))
		Expect(session1.OutputToString()).To(BeEquivalentTo(cid2))
	})
})
