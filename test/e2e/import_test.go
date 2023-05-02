package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman import with source and reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", outfile, "foobar.com/imported-image:latest"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "--type", "image", "foobar.com/imported-image:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
	})

	It("podman import with custom os, arch and variant", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--os", "testos", "--arch", "testarch", outfile, "foobar.com/imported-image:latest"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "--type", "image", "foobar.com/imported-image:latest"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		Expect(results.OutputToString()).To(ContainSubstring("testos"))
		Expect(results.OutputToString()).To(ContainSubstring("testarch"))
	})

	It("podman import without reference", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		// tag the image which proves it is in R/W storage
		tag := podmanTest.Podman([]string{"tag", importImage.OutputToString(), "foo"})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(Exit(0))
	})

	It("podman import with message flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--message", "importing container test message", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"history", "imported-image", "--format", "{{.Comment}}"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		Expect(results.OutputToStringArray()).To(ContainElement(HavePrefix("importing container test message")))
	})

	It("podman import with change flag CMD=<path>", func() {
		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD=/bin/bash", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
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
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD /bin/sh", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
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
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--change", "CMD [\"/bin/bash\"]", outfile, "imported-image"})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).Should(Exit(0))

		results := podmanTest.Podman([]string{"inspect", "imported-image"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(Exit(0))
		imageData := results.InspectImageJSON()
		Expect(imageData[0].Config.Cmd[0]).To(Equal("/bin/bash"))
	})

	It("podman import with signature", func() {
		SkipIfRemote("--signature-policy N/A for remote")

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		export := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		export.WaitWithDefaultTimeout()
		Expect(export).Should(Exit(0))

		importImage := podmanTest.Podman([]string{"import", "--signature-policy", "/no/such/file", outfile})
		importImage.WaitWithDefaultTimeout()
		Expect(importImage).To(ExitWithError())

		result := podmanTest.Podman([]string{"import", "--signature-policy", "/etc/containers/policy.json", outfile})
		result.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(result).To(ExitWithError())
			Expect(result.ErrorToString()).To(ContainSubstring("unknown flag"))
			result := podmanTest.Podman([]string{"import", outfile})
			result.WaitWithDefaultTimeout()
		}
		Expect(result).Should(Exit(0))
	})
})
