package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v3/test/utils"
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
		SkipIfRemote("FIXME: These look like it is supposed to work in remote")
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

		// tag the image which proves it is in R/W storage
		tag := podmanTest.Podman([]string{"tag", importImage.OutputToString(), "foo"})
		tag.WaitWithDefaultTimeout()
		Expect(tag.ExitCode()).To(BeZero())
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
		Expect(results.LineInOutputStartsWith("importing container test message")).To(BeTrue())
	})

	It("podman import with change flag CMD=<path>", func() {
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
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/sh"))
		Expect(imageData[0].Config.Cmd[1]).To(Equal("-c"))
		Expect(imageData[0].Config.Cmd[2]).To(Equal("/bin/bash"))
	})

	It("podman import with change flag CMD <path>", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD /bin/sh", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/sh"))
		Expect(imageData[0].Config.Cmd[1]).To(Equal("-c"))
		Expect(imageData[0].Config.Cmd[2]).To(Equal("/bin/sh"))
	})

	It("podman import with change flag CMD [\"path\",\"path'\"", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD [\"/bin/bash\"]", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Equal(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/bash"))
	})

	It("podman import with signature", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export.ExitCode()).To(Equal(0))

		importImage := podmanTest.Podman([]string{"import", "--signature-policy", "/no/such/file", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage.ExitCode()).To(Not(Equal(0)))

		result := podmanTest.Podman([]string{"import", "--signature-policy", "/etc/containers/policy.json", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})
})
