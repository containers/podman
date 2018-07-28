package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman save", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman save output flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save oci flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save with stdout", func() {
		Skip("Pipe redirection in ginkgo probably wont work")
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", ALPINE, ">", outfile})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bogus image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "FOOBAR"})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Not(Equal(0)))
	})

	It("podman save to directory with oci format", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with v2s2 docker format", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with docker format and compression", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bad filename", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save:colon")

		save := podmanTest.Podman([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Not(Equal(0)))
	})

})
