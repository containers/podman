package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("Podman pull", func() {
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

	It("podman pull from docker with tag", func() {
		session := podmanTest.Podman([]string{"pull", "busybox:glibc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "busybox:glibc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from docker without tag", func() {
		session := podmanTest.Podman([]string{"pull", "busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from alternate registry with tag", func() {
		session := podmanTest.Podman([]string{"pull", "registry.fedoraproject.org/fedora-minimal:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "registry.fedoraproject.org/fedora-minimal:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from alternate registry without tag", func() {
		session := podmanTest.Podman([]string{"pull", "registry.fedoraproject.org/fedora-minimal"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "registry.fedoraproject.org/fedora-minimal"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by digest", func() {
		session := podmanTest.Podman([]string{"pull", "alpine@sha256:1072e499f3f655a032e88542330cf75b02e7bdf673278f701d7ba61629ee3ebe"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "alpine:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull bogus image", func() {
		session := podmanTest.Podman([]string{"pull", "umohnani/get-started"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman pull from docker-archive", func() {
		session := podmanTest.Podman([]string{"save", "-o", "/tmp/alp.tar", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", "docker-archive:/tmp/alp.tar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		clean := podmanTest.SystemExec("rm", []string{"/tmp/alp.tar"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman pull from oci-archive", func() {
		session := podmanTest.Podman([]string{"save", "--format", "oci-archive", "-o", "/tmp/oci-alp.tar", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", "oci-archive:/tmp/oci-alp.tar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		clean := podmanTest.SystemExec("rm", []string{"/tmp/oci-alp.tar"})
		clean.WaitWithDefaultTimeout()
	})

	It("podman pull from local directory", func() {
		setup := podmanTest.SystemExec("mkdir", []string{"-p", "/tmp/podmantestdir"})
		setup.WaitWithDefaultTimeout()
		session := podmanTest.Podman([]string{"push", "alpine", "dir:/tmp/podmantestdir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", "dir:/tmp/podmantestdir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "podmantestdir"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		clean := podmanTest.SystemExec("rm", []string{"-fr", "/tmp/podmantestdir"})
		clean.WaitWithDefaultTimeout()
	})

	It("podman pull from local directory", func() {
		podmanTest.RestoreArtifact(ALPINE)
		setup := podmanTest.Podman([]string{"images", ALPINE, "-q", "--no-trunc"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		shortImageId := strings.Split(setup.OutputToString(), ":")[1]

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		pull := podmanTest.Podman([]string{"pull", "-q", ALPINE})
		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(Equal(0))

		Expect(pull.OutputToString()).To(ContainSubstring(shortImageId))
	})
})
