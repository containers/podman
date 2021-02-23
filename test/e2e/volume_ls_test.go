package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volume ls", func() {
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
		podmanTest.CleanupVolume()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman ls volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
	})

	It("podman ls volume with JSON format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman ls volume with Go template", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "table {{.Name}} {{.Driver}} {{.Scope}}"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1), session.OutputToString())
	})

	It("podman ls volume with --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "foo=bar", "myvol"})
		volName := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=bar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=baz"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(0))
	})

	It("podman volume ls with --filter dangling", func() {
		volName1 := "volume1"
		session := podmanTest.Podman([]string{"volume", "create", volName1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		volName2 := "volume2"
		session2 := podmanTest.Podman([]string{"volume", "create", volName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))

		ctr := podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/test", volName2), ALPINE, "sh"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(Equal(0))

		lsNoDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=false", "--quiet"})
		lsNoDangling.WaitWithDefaultTimeout()
		Expect(lsNoDangling.ExitCode()).To(Equal(0))
		Expect(lsNoDangling.OutputToString()).To(ContainSubstring(volName2))

		lsDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=true", "--quiet"})
		lsDangling.WaitWithDefaultTimeout()
		Expect(lsDangling.ExitCode()).To(Equal(0))
		Expect(lsDangling.OutputToString()).To(ContainSubstring(volName1))
	})
	It("podman ls volume with multiple --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "foo=bar", "myvol"})
		volName := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "foo2=bar2", "anothervol"})
		anotherVol := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo", "--filter", "label=foo2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(anotherVol))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=bar", "--filter", "label=foo2=bar2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(anotherVol))
	})
})
