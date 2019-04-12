// +build !remoteclient

package integration

import (
	"os"

	"fmt"
	"path/filepath"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pull", func() {
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

	It("podman pull from docker a not existing image", func() {
		session := podmanTest.Podman([]string{"pull", "ibetthisdoesntexistthere:foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
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
		session := podmanTest.Podman([]string{"pull", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from alternate registry without tag", func() {
		session := podmanTest.Podman([]string{"pull", "quay.io/libpod/alpine_nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "quay.io/libpod/alpine_nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by digest", func() {
		session := podmanTest.Podman([]string{"pull", "alpine@sha256:1072e499f3f655a032e88542330cf75b02e7bdf673278f701d7ba61629ee3ebe"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "alpine:none"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull bogus image", func() {
		session := podmanTest.Podman([]string{"pull", "umohnani/get-started"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman pull from docker-archive", func() {
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{"save", "-o", tarfn, "alpine"})
		session.WaitWithDefaultTimeout()

		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", fmt.Sprintf("docker-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from oci-archive", func() {
		tarfn := filepath.Join(podmanTest.TempDir, "oci-alp.tar")
		session := podmanTest.Podman([]string{"save", "--format", "oci-archive", "-o", tarfn, "alpine"})
		session.WaitWithDefaultTimeout()

		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", fmt.Sprintf("oci-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from local directory", func() {
		dirpath := filepath.Join(podmanTest.TempDir, "alpine")
		os.MkdirAll(dirpath, os.ModePerm)
		imgPath := fmt.Sprintf("dir:%s", dirpath)

		session := podmanTest.Podman([]string{"push", "alpine", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"pull", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull check quiet", func() {
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

	It("podman pull check all tags", func() {
		session := podmanTest.Podman([]string{"pull", "--all-tags", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOuputStartsWith("Pulled Images:")).To(BeTrue())

		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 4))

		rmi := podmanTest.Podman([]string{"rmi", "-a", "-f"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))
	})
})
