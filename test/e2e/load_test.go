package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		podmanTest.AddImageToRWStore(ALPINE)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman load input flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		fmt.Println(images.OutputToStringArray())

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load compressed tar file", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		compress := SystemExec("gzip", []string{outfile})
		Expect(compress).Should(Exit(0))
		outfile += ".gz"

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load oci-archive image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load oci-archive with signature", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "--signature-policy", "/etc/containers/policy.json", "-i", outfile})
		result.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(result).To(ExitWithError())
			Expect(result.ErrorToString()).To(ContainSubstring("unknown flag"))
			result = podmanTest.Podman([]string{"load", "-i", outfile})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(Exit(0))
		} else {
			Expect(result).Should(Exit(0))
		}
	})

	It("podman load with quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load directory", func() {
		SkipIfRemote("Remote does not support loading directories")
		outdir := filepath.Join(podmanTest.TempDir, "alpine")

		save := podmanTest.Podman([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outdir})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman-remote load directory", func() {
		// Remote-only test looking for the specific remote error
		// message when trying to load a directory.
		if !IsRemote() {
			Skip("Remote only test")
		}

		result := podmanTest.Podman([]string{"load", "-i", podmanTest.TempDir})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))

		errMsg := fmt.Sprintf("remote client supports archives only but %q is a directory", podmanTest.TempDir)
		Expect(result.ErrorToString()).To(ContainSubstring(errMsg))
	})

	It("podman load bogus file", func() {
		save := podmanTest.Podman([]string{"load", "-i", "foobar.tar"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman load multiple tags", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip on ppc64le")
		}
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")
		alpVersion := "quay.io/libpod/alpine:3.2"

		pull := podmanTest.Podman([]string{"pull", alpVersion})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(Exit(0))

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE, alpVersion})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE, alpVersion})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		inspect = podmanTest.Podman([]string{"inspect", alpVersion})
		inspect.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load localhost registry from scratch", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")
		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		load := podmanTest.Podman([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(Exit(0))

		result := podmanTest.Podman([]string{"images", "hello:world"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(Not(ContainSubstring("docker")))
		Expect(result.OutputToString()).To(ContainSubstring("localhost"))
	})

	It("podman load localhost registry from scratch and :latest", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")

		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"rmi", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		load := podmanTest.Podman([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(Exit(0))

		result := podmanTest.Podman([]string{"images", "hello:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(Not(ContainSubstring("docker")))
		Expect(result.OutputToString()).To(ContainSubstring("localhost"))
	})

	It("podman load localhost registry from dir", func() {
		SkipIfRemote("podman-remote does not support loading directories")
		outfile := filepath.Join(podmanTest.TempDir, "load")

		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-dir", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		load := podmanTest.Podman([]string{"load", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(Exit(0))

		result := podmanTest.Podman([]string{"images", "load:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(Not(ContainSubstring("docker")))
		Expect(result.OutputToString()).To(ContainSubstring("localhost"))
	})

	It("podman load xz compressed image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alp.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
		session := SystemExec("xz", []string{outfile})
		Expect(session).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		result := podmanTest.Podman([]string{"load", "-i", outfile + ".xz"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman load multi-image archive", func() {
		result := podmanTest.Podman([]string{"load", "-i", "./testdata/docker-two-images.tar.xz"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("example.com/empty:latest"))
		Expect(result.OutputToString()).To(ContainSubstring("example.com/empty/but:different"))
	})
})
