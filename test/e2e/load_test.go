package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman load", func() {
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
	})

	It("podman load input flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load compressed tar file", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		compress := podmanTest.SystemExec("gzip", []string{outfile})
		compress.WaitWithDefaultTimeout()
		outfile = outfile + ".gz"

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load oci-archive image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load oci-archive with signature", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "--signature-policy", "/etc/containers/policy.json", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load with quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load directory", func() {
		outdir := filepath.Join(podmanTest.TempDir, "alpine")

		save := podmanTest.Podman([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outdir})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load bogus file", func() {
		save := podmanTest.Podman([]string{"load", "-i", "foobar.tar"})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).ToNot(Equal(0))
	})
})
