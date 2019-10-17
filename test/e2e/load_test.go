// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman load", func() {
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

	It("podman load input flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		images := podmanTest.PodmanNoCache([]string{"images"})
		images.WaitWithDefaultTimeout()
		fmt.Println(images.OutputToStringArray())

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load compressed tar file", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		compress := SystemExec("gzip", []string{outfile})
		Expect(compress.ExitCode()).To(Equal(0))
		outfile = outfile + ".gz"

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load oci-archive image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load oci-archive with signature", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "--signature-policy", "/etc/containers/policy.json", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load with quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load directory", func() {
		outdir := filepath.Join(podmanTest.TempDir, "alpine")

		save := podmanTest.PodmanNoCache([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
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
		save := podmanTest.PodmanNoCache([]string{"load", "-i", "foobar.tar"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman load multiple tags", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip on ppc64le")
		}
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")
		alpVersion := "docker.io/library/alpine:3.2"

		pull := podmanTest.PodmanNoCache([]string{"pull", alpVersion})
		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(Equal(0))

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE, alpVersion})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE, alpVersion})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		inspect := podmanTest.PodmanNoCache([]string{"inspect", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		inspect = podmanTest.PodmanNoCache([]string{"inspect", alpVersion})
		inspect.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman load localhost registry from scratch", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")
		setup := podmanTest.PodmanNoCache([]string{"tag", ALPINE, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		load := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"images", "hello:world"})
		result.WaitWithDefaultTimeout()
		Expect(result.LineInOutputContains("docker")).To(Not(BeTrue()))
		Expect(result.LineInOutputContains("localhost")).To(BeTrue())
	})

	It("podman load localhost registry from scratch and :latest", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")

		setup := podmanTest.PodmanNoCache([]string{"tag", "fedora-minimal", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"rmi", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		load := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"images", "hello:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.LineInOutputContains("docker")).To(Not(BeTrue()))
		Expect(result.LineInOutputContains("localhost")).To(BeTrue())
	})

	It("podman load localhost registry from dir", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load")

		setup := podmanTest.PodmanNoCache([]string{"tag", BB, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-dir", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		setup = podmanTest.PodmanNoCache([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		load := podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"images", "load:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.LineInOutputContains("docker")).To(Not(BeTrue()))
		Expect(result.LineInOutputContains("localhost")).To(BeTrue())
	})

	It("podman load xz compressed image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "bb.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, BB})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
		session := SystemExec("xz", []string{outfile})
		Expect(session.ExitCode()).To(Equal(0))

		rmi := podmanTest.PodmanNoCache([]string{"rmi", BB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		result := podmanTest.PodmanNoCache([]string{"load", "-i", outfile + ".xz"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
