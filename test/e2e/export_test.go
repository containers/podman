//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman export", func() {

	It("podman export output flag", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		outfile := filepath.Join(podmanTest.TempDir, "container.tar")
		result := podmanTest.Podman([]string{"export", "-o", outfile, cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
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
		Expect(result).Should(ExitCleanly())
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
		Expect(result).To(ExitWithError(125, "invalid filename (should not contain ':')"))
	})
})
