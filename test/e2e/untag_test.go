package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman untag all", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		tags := []string{CIRROS_IMAGE, "registry.com/foo:bar", "localhost/foo:bar"}

		cmd := []string{"tag"}
		cmd = append(cmd, tags...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Make sure that all tags exists.
		for _, t := range tags {
			session = podmanTest.Podman([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}

		// No arguments -> remove all tags.
		session = podmanTest.Podman([]string{"untag", CIRROS_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Make sure that none of tags exists anymore.
		for _, t := range tags {
			session = podmanTest.Podman([]string{"image", "exists", t})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(1))
		}
	})

	It("podman tag/untag - tag normalization", func() {
		podmanTest.AddImageToRWStore(CIRROS_IMAGE)

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
			session := podmanTest.Podman([]string{"tag", CIRROS_IMAGE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"untag", CIRROS_IMAGE, tt.tag})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"image", "exists", tt.normalized})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(1))
		}
	})

})
