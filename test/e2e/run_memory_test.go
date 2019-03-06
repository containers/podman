// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run memory", func() {
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman run memory test", func() {
		session := podmanTest.Podman([]string{"run", "--memory=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("41943040"))
	})

	It("podman run memory-reservation test", func() {
		if podmanTest.Host.Distribution == "ubuntu" {
			Skip("Unable to perform test on Ubuntu distributions due to memory management")
		}
		session := podmanTest.Podman([]string{"run", "--memory-reservation=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.soft_limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("41943040"))
	})

	It("podman run memory-swappiness test", func() {
		session := podmanTest.Podman([]string{"run", "--memory-swappiness=15", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.swappiness"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("15"))
	})

	It("podman run kernel-memory test", func() {
		session := podmanTest.Podman([]string{"run", "--kernel-memory=40m", ALPINE, "cat", "/sys/fs/cgroup/memory/memory.kmem.limit_in_bytes"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("41943040"))
	})
})
