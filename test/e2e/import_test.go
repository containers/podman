package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman import", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman import with source and reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", outfile, "foobar.com/imported-image:latest"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "--type", "image", "foobar.com/imported-image:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
	})

	It("podman import without reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"images", "-q"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("podman import with message flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", "--message", "importing container test message", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"history", "imported-image", "--format", "{{.Comment}}"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.LineInOuputStartsWith("importing container test message")).To(BeTrue())
	})

	It("podman import with change flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD=/bin/bash", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		imageData := results.InspectImageJSON()
		Expect(imageData[0].ContainerConfig.Cmd[0]).To(Equal("/bin/bash"))
	})

})
