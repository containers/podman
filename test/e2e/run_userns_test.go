package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman UserNS support", func() {
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

	It("podman uidmapping and gidmapping", func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}

		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:70000", "--gidmap=0:20000:70000", "busybox", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

	It("podman uidmapping and gidmapping --net=host", func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}
		session := podmanTest.Podman([]string{"run", "--net=host", "--uidmap=0:1:70000", "--gidmap=0:20000:70000", "busybox", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

})
