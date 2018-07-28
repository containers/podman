package integration

import (
	"os"

	"fmt"
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
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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

	// It essentially repeats the test above but with the `-it` short option
	// that broke execution at:
	//     https://github.com/projectatomic/libpod/pull/1066#issuecomment-403562116
	// To avoid a potential future regression, use this as a test.
	It("podman uidmapping and gidmapping with short-opts", func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}

		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:70000", "--gidmap=0:20000:70000", "-it", "busybox", "echo", "hello"})
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
