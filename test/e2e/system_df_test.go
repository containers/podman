package integration

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system df", func() {
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

	It("podman system df", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", "--name", "container1", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		totImages := strconv.Itoa(len(session.OutputToStringArray()))

		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(4))
		images := strings.Fields(session.OutputToStringArray()[1])
		containers := strings.Fields(session.OutputToStringArray()[2])
		volumes := strings.Fields(session.OutputToStringArray()[3])
		Expect(images[1]).To(Equal(string(totImages)))
		Expect(containers[1]).To(Equal("2"))
		Expect(volumes[2]).To(Equal("1"))
	})

	It("podman system df image with no tag", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"image", "untag", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
