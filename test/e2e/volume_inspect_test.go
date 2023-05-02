package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volume inspect", func() {
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
		podmanTest.CleanupVolume()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman inspect volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "inspect", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman inspect volume with Go format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Name}}", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(volName))
	})

	It("podman inspect volume with --all flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol1"})
		session.WaitWithDefaultTimeout()
		volName1 := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "myvol2"})
		session.WaitWithDefaultTimeout()
		volName2 := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Name}}", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[0]).To(Equal(volName1))
		Expect(session.OutputToStringArray()[1]).To(Equal(volName2))
	})

	It("inspect volume finds options", func() {
		volName := "testvol"
		session := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"volume", "inspect", volName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("tmpfs"))
	})
})
