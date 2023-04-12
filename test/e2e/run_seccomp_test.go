package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run", func() {
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

	It("podman run --seccomp-policy default", func() {
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "default", alpineSeccomp, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run --seccomp-policy ''", func() {
		// Empty string is interpreted as "default".
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "", alpineSeccomp, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run --seccomp-policy invalid", func() {
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "invalid", alpineSeccomp, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run --seccomp-policy image (block all syscalls)", func() {
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "image", alpineSeccomp, "ls"})
		session.WaitWithDefaultTimeout()
		// TODO: we're getting a "cannot start a container that has
		//       stopped" error which seems surprising.  Investigate
		//       why that is so.
		Expect(session).To(ExitWithError())
	})

	It("podman run --seccomp-policy image (bogus profile)", func() {
		session := podmanTest.Podman([]string{"run", "--seccomp-policy", "image", alpineBogusSeccomp, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})
})
