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

	It("podman load multiple tags", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")
		alpVersion := "docker.io/library/alpine:3.2"

		pull := podmanTest.Podman([]string{"pull", alpVersion})
		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(Equal(0))

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE, alpVersion})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE, alpVersion})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		inspect = podmanTest.Podman([]string{"inspect", alpVersion})
		inspect.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load localhost repo from scratch", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")
		setup := podmanTest.Podman([]string{"pull", fedoraMinimal})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"tag", "fedora-minimal", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		load := podmanTest.Podman([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"images", "-f", "label", "hello:world"})
		result.WaitWithDefaultTimeout()
		Expect(result.LineInOutputContains("docker")).To(Not(BeTrue()))
		Expect(result.LineInOutputContains("localhost")).To(BeTrue())
	})

	It("podman load localhost repo from dir", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load")
		setup := podmanTest.Podman([]string{"pull", fedoraMinimal})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"tag", "fedora-minimal", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-dir", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		load := podmanTest.Podman([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"images", "-f", "label", "load:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.LineInOutputContains("docker")).To(Not(BeTrue()))
		Expect(result.LineInOutputContains("localhost")).To(BeTrue())
	})

	It("podman load xz compressed image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
		session := podmanTest.SystemExec("xz", []string{outfile})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile + ".xz"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
