package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman untag", func() {
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
		podmanTest.RestoreAllArtifacts()

		for _, tag := range []string{"test", "foo", "bar"} {
			session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, tag})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
		}

	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman untag all", func() {
		Skip(v2remotefail)
		session := podmanTest.PodmanNoCache([]string{"untag", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"images", ALPINE})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman untag single", func() {
		session := podmanTest.PodmanNoCache([]string{"untag", ALPINE, "localhost/test:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		results := podmanTest.PodmanNoCache([]string{"images"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.OutputToStringArray()).To(HaveLen(5))
		Expect(results.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(results.LineInOuputStartsWith("localhost/foo")).To(BeTrue())
		Expect(results.LineInOuputStartsWith("localhost/bar")).To(BeTrue())
		Expect(results.LineInOuputStartsWith("localhost/test")).To(BeFalse())
	})

	It("podman untag not enough arguments", func() {
		session := podmanTest.PodmanNoCache([]string{"untag"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).NotTo(Equal(0))
	})
})
