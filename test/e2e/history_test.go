package integration

import (
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman history", func() {
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

	It("podman history output flag", func() {
		session := podmanTest.Podman([]string{"history", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman history with GO template", func() {
		session := podmanTest.Podman([]string{"history", "--format", "{{.ID}} {{.Created}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman history with human flag", func() {
		session := podmanTest.Podman([]string{"history", "--human=false", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman history with quiet flag", func() {
		session := podmanTest.Podman([]string{"history", "-qH", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))
	})

	It("podman history with no-trunc flag", func() {
		session := podmanTest.Podman([]string{"history", "--no-trunc", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 0))

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.ID}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		lines := session.OutputToStringArray()
		Expect(len(lines)).To(BeNumerically(">", 0))
		// the image id must be 64 chars long
		Expect(len(lines[0])).To(BeNumerically("==", 64))

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.CreatedBy}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		lines = session.OutputToStringArray()
		Expect(len(lines)).To(BeNumerically(">", 0))
		Expect(session.OutputToString()).ToNot(ContainSubstring("..."))
		// the second line in the alpine history contains a command longer than 45 chars
		Expect(len(lines[1])).To(BeNumerically(">", 45))
	})

	It("podman history with json flag", func() {
		session := podmanTest.Podman([]string{"history", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})
})
