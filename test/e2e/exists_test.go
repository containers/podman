package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman image exists in local storage by fq name", func() {
		session := podmanTest.Podman([]string{"image", "exists", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman image exists in local storage by short name", func() {
		session := podmanTest.Podman([]string{"image", "exists", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman image does not exist in local storage", func() {
		session := podmanTest.Podman([]string{"image", "exists", "alpine9999"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})
	It("podman container exists in local storage by name", func() {
		setup := podmanTest.RunTopContainer("foobar")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"container", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman container exists in local storage by container ID", func() {
		setup := podmanTest.RunTopContainer("")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cid := setup.OutputToString()

		session := podmanTest.Podman([]string{"container", "exists", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman container exists in local storage by short container ID", func() {
		setup := podmanTest.RunTopContainer("")
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))
		cid := setup.OutputToString()[0:12]

		session := podmanTest.Podman([]string{"container", "exists", cid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman container does not exist in local storage", func() {
		session := podmanTest.Podman([]string{"container", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman pod exists in local storage by name", func() {
		setup, _, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar"}})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"pod", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman pod exists in local storage by container ID", func() {
		setup, _, podID := podmanTest.CreatePod(nil)
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"pod", "exists", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman pod exists in local storage by short container ID", func() {
		setup, _, podID := podmanTest.CreatePod(nil)
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		session := podmanTest.Podman([]string{"pod", "exists", podID[0:12]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman pod does not exist in local storage", func() {
		// The exit code for non-existing pod is incorrect (125 vs 1)
		session := podmanTest.Podman([]string{"pod", "exists", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})
})
