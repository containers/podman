package integration

import (
	"os"

	. "github.com/containers/podman/v2/test/utils"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman untag all", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		setup := podmanTest.PodmanNoCache([]string{"pull", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		tags := []string{ALPINE, "registry.com/foo:bar", "localhost/foo:bar"}

		cmd := []string{"tag"}
		cmd = append(cmd, tags...)
		session := podmanTest.PodmanNoCache(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Make sure that all tags exists.
		for _, t := range tags {
			session = podmanTest.PodmanNoCache([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
		}

		// No arguments -> remove all tags.
		session = podmanTest.PodmanNoCache([]string{"untag", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Make sure that none of tags exists anymore.
		for _, t := range tags {
			session = podmanTest.PodmanNoCache([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(1))
		}
	})

	It("podman tag/untag - tag normalization", func() {
		setup := podmanTest.PodmanNoCache([]string{"pull", ALPINE})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		tests := []struct {
			tag, normalized string
		}{
			{"registry.com/image:latest", "registry.com/image:latest"},
			{"registry.com/image", "registry.com/image:latest"},
			{"image:latest", "localhost/image:latest"},
			{"image", "localhost/image:latest"},
		}

		// Make sure that the user input is normalized correctly for
		// `podman tag` and `podman untag`.
		for _, tt := range tests {
			session := podmanTest.PodmanNoCache([]string{"tag", ALPINE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			session = podmanTest.PodmanNoCache([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			session = podmanTest.PodmanNoCache([]string{"untag", ALPINE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			session = podmanTest.PodmanNoCache([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(1))
		}
	})

})
