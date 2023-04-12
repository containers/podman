package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman export", func() {
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

	It("podman export output flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		_, err := os.Stat(outfile)
		Expect(err).ToNot(HaveOccurred())

		err = os.Remove(outfile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman container export output flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"container", "export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		_, err := os.Stat(outfile)
		Expect(err).ToNot(HaveOccurred())

		err = os.Remove(outfile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman export bad filename", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container:with:colon.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})
})
