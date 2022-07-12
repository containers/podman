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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		_, _ = GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman system df", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// run two containers with volumes to create something in the volume
		session = podmanTest.Podman([]string{"run", "-v", "data1:/data", "--name", "container1", BB, "sh", "-c", "echo test > /data/1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "-v", "data2:/data", "--name", "container2", BB, "sh", "-c", "echo test > /data/1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// remove one container, we keep the volume
		session = podmanTest.Podman([]string{"rm", "container2"})
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
		Expect(images[1]).To(Equal(totImages), "total images expected")
		Expect(containers[1]).To(Equal("2"), "total containers expected")
		Expect(volumes[2]).To(Equal("2"), "total volumes expected")
		Expect(volumes[6]).To(Equal("(50%)"), "percentage usage expected")

		session = podmanTest.Podman([]string{"rm", "container1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"system", "df"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		volumes = strings.Fields(session.OutputToStringArray()[3])
		// percentages on volumes were being calculated incorrectly. Make sure we only report 100% and not above
		Expect(volumes[6]).To(Equal("(100%)"), "percentage usage expected")

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

	It("podman system df --format \"{{ json . }}\"", func() {
		session := podmanTest.Podman([]string{"create", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"system", "df", "--format", "{{ json . }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains("Size"))
		Expect(session.LineInOutputContains("Reclaimable"))
		Expect(session.IsJSONOutputValid())
	})
})
