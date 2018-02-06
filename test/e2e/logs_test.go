package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman logs", func() {
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

	It("podman logs for container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		results := podmanTest.Podman([]string{"logs", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
	})

	It("podman logs tail three lines", func() {
		Skip("Tail is not working correctly")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		results := podmanTest.Podman([]string{"logs", "--tail", "3", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(len(results.OutputToStringArray())).To(Equal(3))
	})

	It("podman logs since a given time", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
	})

})
