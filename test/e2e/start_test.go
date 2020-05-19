package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman start", func() {
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

	It("podman start bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman start single container by id", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman container start single container by id", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"container", "create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"container", "start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(cid))
	})

	It("podman container start single container by short id", func() {
		Skip(v2remotefail)
		session := podmanTest.Podman([]string{"container", "create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		shortID := cid[0:10]
		session = podmanTest.Podman([]string{"container", "start", shortID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal(shortID))
	})

	It("podman start single container by name", func() {
		name := "foobar99"
		session := podmanTest.Podman([]string{"create", "-d", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		if podmanTest.RemoteTest {
			Skip("Container-start name check doesn't work on remote client. It always returns the full ID.")
		}
		Expect(session.OutputToString()).To(Equal(name))
	})

	It("podman start multiple containers", func() {
		session := podmanTest.Podman([]string{"create", "-d", "--name", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()
		session2 := podmanTest.Podman([]string{"create", "-d", "--name", "foobar100", ALPINE, "ls"})
		session2.WaitWithDefaultTimeout()
		cid2 := session2.OutputToString()
		session = podmanTest.Podman([]string{"start", cid1, cid2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman start multiple containers with bogus", func() {
		session := podmanTest.Podman([]string{"create", "-d", "--name", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		cid1 := session.OutputToString()
		session = podmanTest.Podman([]string{"start", cid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman multiple containers -- attach should fail", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foobar1", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"create", "--name", "foobar2", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "-a", "foobar1", "foobar2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman failed to start with --rm should delete the container", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"create", "-it", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		start := podmanTest.Podman([]string{"start", "-l"})
		start.WaitWithDefaultTimeout()
		Expect(start).To(ExitWithError())

		Eventually(podmanTest.NumberOfContainers(), defaultWaitTimeout, 3.0).Should(BeZero())
	})

	It("podman failed to start without --rm should NOT delete the container", func() {
		SkipIfRemote()
		session := podmanTest.Podman([]string{"create", "-it", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		start := podmanTest.Podman([]string{"start", "-l"})
		start.WaitWithDefaultTimeout()
		Expect(start).To(ExitWithError())

		Eventually(podmanTest.NumberOfContainers(), defaultWaitTimeout, 3.0).Should(Equal(1))
	})

	It("podman start --sig-proxy should not work without --attach", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"start", "-l", "--sig-proxy"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})
})
