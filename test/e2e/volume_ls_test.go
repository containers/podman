package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
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
	})

	AfterEach(func() {
		podmanTest.CleanupVolume()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman ls volume", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman ls volume filter with a key pattern", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "helloworld=world", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=hello*"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman ls volume with JSON format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman ls volume with Go template", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "table {{.Name}} {{.Driver}} {{.Scope}}"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(Exit(0))
		arr := session.OutputToStringArray()
		Expect(arr).To(HaveLen(2))
		Expect(arr[0]).To(ContainSubstring("NAME"))
		Expect(arr[1]).To(ContainSubstring("myvol"))
	})

	It("podman ls volume with --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "foo=bar", "myvol"})
		volName := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=baz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman ls volume with --filter until flag", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "until=5000000000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "until=50000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())
	})

	It("podman volume ls with --filter dangling", func() {
		volName1 := "volume1"
		session := podmanTest.Podman([]string{"volume", "create", volName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		volName2 := "volume2"
		session2 := podmanTest.Podman([]string{"volume", "create", volName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		ctr := podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/test", volName2), ALPINE, "sh"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		lsNoDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=false", "--quiet"})
		lsNoDangling.WaitWithDefaultTimeout()
		Expect(lsNoDangling).Should(Exit(0))
		Expect(lsNoDangling.OutputToString()).To(ContainSubstring(volName2))

		lsDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=true", "--quiet"})
		lsDangling.WaitWithDefaultTimeout()
		Expect(lsDangling).Should(Exit(0))
		Expect(lsDangling.OutputToString()).To(ContainSubstring(volName1))
	})

	It("podman ls volume with --filter name", func() {
		volName1 := "volume1"
		session := podmanTest.Podman([]string{"volume", "create", volName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		volName2 := "volume2"
		session2 := podmanTest.Podman([]string{"volume", "create", volName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volume1*"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName1))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(volName2))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volumex"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volume1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName1))
	})

	It("podman ls volume with multiple --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "foo=bar", "myvol"})
		volName := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "--label", "foo2=bar2", "anothervol"})
		anotherVol := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo", "--filter", "label=foo2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(anotherVol))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=bar", "--filter", "label=foo2=bar2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(anotherVol))
	})
})
