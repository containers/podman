package integration

import (
	"fmt"
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system reset", func() {
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
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman system reset", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		// system reset will not remove additional store images, so need to grab length

		// change the network dir so that we do not conflict with other tests
		// that would use the same network dir and cause unnecessary flakes
		podmanTest.NetworkConfigDir = tempdir

		session := podmanTest.Podman([]string{"rmi", "--force", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		l := len(session.OutputToStringArray())

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		Expect(session.ErrorToString()).To(Not(ContainSubstring("Failed to add pause process")))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(l))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"container", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"network", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// default network should exists
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		var f *os.File
		f, err = os.Create("tempStorage.conf")
		if err != nil {
			os.Exit(1)
		}

		env := os.Getenv("CONTAINERS_STORAGE_CONF")
		err = os.Setenv("CONTAINERS_STORAGE_CONF", "tempStorage.conf")
		if err != nil {
			os.Exit(1)
		}

		_, err = f.WriteString("[storage]\ndriver = \"overlay\"\nrunroot = \"" + podmanTest.RunRoot + "\"\ngraphroot = \"" + podmanTest.Root + "\"")
		Expect(err).Should(BeNil())

		session = podmanTest.Podman([]string{"system", "reset", "-f", "--run-root", "foo", "--graph-root", "bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		content, _ := os.ReadFile(f.Name())
		Expect(string(content[:])).Should(ContainSubstring("foo"))
		Expect(string(content[:])).Should(ContainSubstring("bar"))

		err = os.Setenv("CONTAINERS_STORAGE_CONF", env)
		Expect(err).Should(BeNil())
		err = os.Remove("tempStorage.conf")
		Expect(err).Should(BeNil())

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})
})
