package integration

import (
	"os"

	"github.com/containers/libpod/libpod/define"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run exit", func() {
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

	It("podman run exit define.ExecErrorCodeGeneric", func() {
		result := podmanTest.Podman([]string{"run", "--foobar", ALPINE, "ls", "$tmp"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(define.ExecErrorCodeGeneric))
	})

	It("podman run exit ExecErrorCodeCannotInvoke", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(define.ExecErrorCodeCannotInvoke))
	})

	It("podman run exit ExecErrorCodeNotFound", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Not(Equal(define.ExecErrorCodeGeneric)))
		// TODO This is failing we believe because of a race condition
		// Between conmon and podman closing the socket early.
		// Test with the following, once the race condition is solved
		// Expect(result.ExitCode()).To(Equal(define.ExecErrorCodeNotFound))
	})

	It("podman run exit 0", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman run exit 50", func() {
		Skip(v2remotefail)
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 50"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(50))
	})
})
