package integration

import (
	"fmt"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman image|container exists", func() {
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
	It("podman image exists in local storage by fq name", func() {
		session := podmanTest.Podman([]string{"image", "exists", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman image exists in local storage by short name", func() {
		session := podmanTest.Podman([]string{"image", "exists", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman image does not exist in local storage", func() {
		session := podmanTest.Podman([]string{"image", "exists", "alpine9999"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})
	It("podman container exists in local storage by name", func() {
		setup := podmanTest.RunTopContainer("foobar")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"container", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman container exists in local storage by container ID", func() {
		setup := podmanTest.RunTopContainer("")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		session := podmanTest.Podman([]string{"container", "exists", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman container exists in local storage by short container ID", func() {
		setup := podmanTest.RunTopContainer("")
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()[0:12]

		session := podmanTest.Podman([]string{"container", "exists", cid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman container does not exist in local storage", func() {
		session := podmanTest.Podman([]string{"container", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("podman pod exists in local storage by name", func() {
		setup, rc, _ := podmanTest.CreatePod("foobar")
		setup.WaitWithDefaultTimeout()
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman pod exists in local storage by container ID", func() {
		setup, rc, podID := podmanTest.CreatePod("")
		setup.WaitWithDefaultTimeout()
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "exists", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman pod exists in local storage by short container ID", func() {
		setup, rc, podID := podmanTest.CreatePod("")
		setup.WaitWithDefaultTimeout()
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "exists", podID[0:12]})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman pod does not exist in local storage", func() {
		session := podmanTest.Podman([]string{"pod", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})
})
