package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman top", func() {
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

	It("podman top without container name or id", func() {
		result := podmanTest.Podman([]string{"top"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on bogus container", func() {
		result := podmanTest.Podman([]string{"top", "1234"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on non-running container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"top", cid})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

	It("podman top on container", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"top", "-l"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	//      XXX(ps-issue): for the time being, podman-top and the libpod API
	//      GetContainerPidInformation(...) will ignore any arguments passed to ps,
	//      so we have to disable the tests below.  Please refer to
	//      https://github.com/projectatomic/libpod/pull/939 for more background
	//      information.

	It("podman top with options", func() {
		Skip("podman-top with options: options are temporarily ignored")
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"top", session.OutputToString(), "-o", "pid,fuser,f,comm,label"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman top on container invalid options", func() {
		Skip("podman-top with invalid options: options are temporarily ignored")
		top := podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(Equal(0))
		cid := top.OutputToString()

		result := podmanTest.Podman([]string{"top", cid, "-o time"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(125))
	})

})
