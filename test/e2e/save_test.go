package integration

import (
	"os"
	"path/filepath"

	"github.com/containers/libpod/pkg/rootless"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman save", func() {
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

	It("podman save output flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save oci flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save with stdout", func() {
		Skip("Pipe redirection in ginkgo probably won't work")
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", ALPINE, ">", outfile})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bogus image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "FOOBAR"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save to directory with oci format", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with v2s2 docker format", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with docker format and compression", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bad filename", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save:colon")

		save := podmanTest.PodmanNoCache([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save image with digest reference", func() {
		// pull a digest reference
		session := podmanTest.PodmanNoCache([]string{"pull", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// save a digest reference should exit without error.
		outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINELISTDIGEST})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})
})
