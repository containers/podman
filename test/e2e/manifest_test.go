package integration

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman manifest", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	const (
		imageList                      = "docker://k8s.gcr.io/pause:3.1"
		imageListInstance              = "docker://k8s.gcr.io/pause@sha256:f365626a556e58189fc21d099fc64603db0f440bff07f77c740989515c544a39"
		imageListARM64InstanceDigest   = "sha256:f365626a556e58189fc21d099fc64603db0f440bff07f77c740989515c544a39"
		imageListAMD64InstanceDigest   = "sha256:59eec8837a4d942cc19a52b8c09ea75121acc38114a2c68b98983ce9356b8610"
		imageListARMInstanceDigest     = "sha256:c84b0a3a07b628bc4d62e5047d0f8dff80f7c00979e1e28a821a033ecda8fe53"
		imageListPPC64LEInstanceDigest = "sha256:bcf9771c0b505e68c65440474179592ffdfa98790eb54ffbf129969c5e429990"
		imageListS390XInstanceDigest   = "sha256:882a20ee0df7399a445285361d38b711c299ca093af978217112c73803546d5e"
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
	It("create w/o image", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("create w/ image", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("inspect", func() {
		session := podmanTest.Podman([]string{"manifest", "inspect", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "inspect", "quay.io/libpod/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// inspect manifest of single image
		session = podmanTest.Podman([]string{"manifest", "inspect", "quay.io/libpod/busybox@sha256:6655df04a3df853b029a5fac8836035ac4fab117800c9a6c4b69341bb5306c3d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("add w/ inspect", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		id := strings.TrimSpace(string(session.Out.Contents()))

		session = podmanTest.Podman([]string{"manifest", "inspect", id})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "--arch=arm64", "foo", imageListInstance})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageListARM64InstanceDigest))
	})

	It("tag", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foobar", "quay.io/libpod/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"tag", "foobar", "foobar2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session2 := podmanTest.Podman([]string{"manifest", "inspect", "foobar2"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		Expect(session2.OutputToString()).To(Equal(session.OutputToString()))
	})

	It(" add --all", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring(imageListAMD64InstanceDigest),
				ContainSubstring(imageListARMInstanceDigest),
				ContainSubstring(imageListARM64InstanceDigest),
				ContainSubstring(imageListPPC64LEInstanceDigest),
				ContainSubstring(imageListS390XInstanceDigest),
			))
	})

	It("add --os", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--os", "bar", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(`"os": "bar"`))
	})

	It("annotate", func() {
		SkipIfRemote("Not supporting annotate on remote connections")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageListInstance})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "annotate", "--arch", "bar", "foo", imageListARM64InstanceDigest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(`"architecture": "bar"`))
	})

	It("remove digest", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageListARM64InstanceDigest))
		session = podmanTest.Podman([]string{"manifest", "remove", "foo", imageListARM64InstanceDigest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring(imageListAMD64InstanceDigest),
				ContainSubstring(imageListARMInstanceDigest),
				ContainSubstring(imageListPPC64LEInstanceDigest),
				ContainSubstring(imageListS390XInstanceDigest),
				Not(
					ContainSubstring(imageListARM64InstanceDigest)),
			))
	})

	It("remove not-found", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "remove", "foo", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"manifest", "rm", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("push --all", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).To(BeNil())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"manifest", "push", "--all", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		files, err := filepath.Glob(dest + string(os.PathSeparator) + "*")
		Expect(err).ShouldNot(HaveOccurred())
		check := SystemExec("sha256sum", files)
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		prefix := "sha256:"
		Expect(check.OutputToString()).To(
			And(
				ContainSubstring(strings.TrimPrefix(imageListAMD64InstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARMInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListPPC64LEInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListS390XInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARM64InstanceDigest, prefix)),
			))
	})

	It("push", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).To(BeNil())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"push", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		files, err := filepath.Glob(dest + string(os.PathSeparator) + "*")
		Expect(err).To(BeNil())
		check := SystemExec("sha256sum", files)
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))

		prefix := "sha256:"
		Expect(check.OutputToString()).To(
			And(
				ContainSubstring(strings.TrimPrefix(imageListAMD64InstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARMInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListPPC64LEInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListS390XInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARM64InstanceDigest, prefix)),
			))
	})

	It("push --rm", func() {
		SkipIfRemote("remote does not support --rm")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).To(BeNil())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"manifest", "push", "--purge", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"manifest", "rm", "foo1", "foo2"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("exists", func() {
		manifestList := "manifest-list"
		session := podmanTest.Podman([]string{"manifest", "create", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "exists", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "exists", "no-manifest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("rm should not remove referenced images", func() {
		manifestList := "manifestlist"
		imageName := "quay.io/libpod/busybox"

		session := podmanTest.Podman([]string{"pull", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "create", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", manifestList, imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "rm", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// image should still show up
		session = podmanTest.Podman([]string{"image", "exists", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
