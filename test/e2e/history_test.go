package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman history output flag", func() {
		session := podmanTest.Podman([]string{"history", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with GO template", func() {
		session := podmanTest.Podman([]string{"history", "--format", "{{.ID}} {{.Created}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with human flag", func() {
		session := podmanTest.Podman([]string{"history", "--human=false", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with quiet flag", func() {
		session := podmanTest.Podman([]string{"history", "-qH", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with no-trunc flag", func() {
		session := podmanTest.Podman([]string{"history", "--no-trunc", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.ID}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		lines := session.OutputToStringArray()
		Expect(lines).ToNot(BeEmpty())
		// the image id must be 64 chars long
		Expect(lines[0]).To(HaveLen(64))

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.CreatedBy}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		lines = session.OutputToStringArray()
		Expect(lines).ToNot(BeEmpty())
		Expect(session.OutputToString()).ToNot(ContainSubstring("..."))
		// the second line in the alpine history contains a command longer than 45 chars
		Expect(len(lines[1])).To(BeNumerically(">", 45))
	})

	It("podman history with json flag", func() {
		session := podmanTest.Podman([]string{"history", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})
})
