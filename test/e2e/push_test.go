package integration

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman push", func() {
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

	It("podman push to containers/storage", func() {
		session := podmanTest.Podman([]string{"push", ALPINE, "containers-storage:busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"rmi", "busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"rmi", "-f", "busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	// push to oci-archive, docker-archive, and dir are tested in pull_test.go

	It("podman push to containers/storage", func() {
		session := podmanTest.Podman([]string{"push", "--remove-signatures", ALPINE, "dir:/tmp/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		clean := podmanTest.SystemExec("rm", []string{"-fr", "/tmp/busybox"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to local registry", func() {
		session := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "docker.io/library/registry:2", "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Give the registry 5 seconds to warm up before pushing
		time.Sleep(5 * time.Second)

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
	})
})
