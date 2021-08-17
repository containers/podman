package integration

import (
	"os"
	"os/exec"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run device", func() {
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
		processTestResult(f)

	})

	It("podman run bad device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/baddevice", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg", ALPINE, "test", "-c", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run device rename test", func() {
		// TODO: Confirm absence of /dev/kmsg in container
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run device permission test", func() {
		// TODO: Confirm write-permission failure
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:r", ALPINE, "test", "-r", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run device rename and permission test", func() {
		// TODO: Confirm write-permission failure
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1:r", ALPINE, "test", "-r", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman run device rename and bad permission test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1:rd", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run device host device and container device parameter are directories", func() {
		SkipIfRootless("Cannot create devices in /dev in rootless mode")
		Expect(os.MkdirAll("/dev/foodevdir", os.ModePerm)).To(BeNil())
		defer os.RemoveAll("/dev/foodevdir")

		mknod := SystemExec("mknod", []string{"/dev/foodevdir/null", "c", "1", "3"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/foodevdir:/dev/bar", ALPINE, "stat", "-c%t:%T", "/dev/bar/null"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal("1:3"))
	})

	It("podman run device host device with --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "test", "-c", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// verify --privileged is required
		session2 := podmanTest.Podman([]string{"run", ALPINE, "test", "-c", "/dev/kmsg"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should((Exit(1)))
	})

	It("podman run CDI device test", func() {
		SkipIfRootless("Rootless will not be able to create files/folders in /etc")
		cdiDir := "/etc/cdi"
		if _, err := os.Stat(cdiDir); os.IsNotExist(err) {
			Expect(os.MkdirAll(cdiDir, os.ModePerm)).To(BeNil())
		}
		defer os.RemoveAll(cdiDir)

		cmd := exec.Command("cp", "cdi/device.json", cdiDir)
		err = cmd.Run()
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "myKmsg", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run --gpus noop", func() {
		session := podmanTest.Podman([]string{"run", "--gpus", "all", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
