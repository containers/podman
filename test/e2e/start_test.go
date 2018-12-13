package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman start bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman start single container by id", func() {
		session := podmanTest.Podman([]string{"create", "-d", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		cid := session.OutputToString()
		session = podmanTest.Podman([]string{"start", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman start single container by name", func() {
		session := podmanTest.Podman([]string{"create", "-d", "--name", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"start", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
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
		session := podmanTest.Podman([]string{"create", "-it", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		start := podmanTest.Podman([]string{"start", "-l"})
		start.WaitWithDefaultTimeout()
		Expect(start.ExitCode()).To(Not(Equal(0)))

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(BeZero())
	})

	It("podman failed to start without --rm should NOT delete the container", func() {
		session := podmanTest.Podman([]string{"create", "-it", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		start := podmanTest.Podman([]string{"start", "-l"})
		start.WaitWithDefaultTimeout()
		Expect(start.ExitCode()).To(Not(Equal(0)))

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(1))
	})
})
