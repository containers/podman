package integration

import (
	"os"

	"fmt"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v2/test/utils"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman pull from docker a not existing image", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "ibetthisdoesntexistthere:foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pull from docker with tag", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "busybox:glibc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "busybox:glibc"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from docker without tag", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from alternate registry with tag", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", nginx})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from alternate registry without tag", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "quay.io/libpod/alpine_nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "quay.io/libpod/alpine_nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by digest", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "alpine@sha256:1072e499f3f655a032e88542330cf75b02e7bdf673278f701d7ba61629ee3ebe"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine:none"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by digest (image list)", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "--override-arch=arm64", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by instance digest (image list)", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "--override-arch=arm64", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(Not(ContainSubstring(ALPINELISTDIGEST)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(Not(ContainSubstring(ALPINELISTDIGEST)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the digest of the instance
		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull by tag (image list)", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "--override-arch=arm64", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// inspect using the tag we used for pulling
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the tag we used for pulling
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the digest of the list
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the image ID
		session = podmanTest.PodmanNoCache([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the tag
		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull bogus image", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "umohnani/get-started"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pull from docker-archive", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		podmanTest.RestoreArtifact(ALPINE)
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.PodmanNoCache([]string{"save", "-o", tarfn, "alpine"})
		session.WaitWithDefaultTimeout()

		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Pulling a multi-image archive without further specifying
		// which image _must_ error out. Pulling is restricted to one
		// image.
		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		expectedError := "Unexpected tar manifest.json: expected 1 item, got 2"
		found, _ := session.ErrorGrepString(expectedError)
		Expect(found).To(Equal(true))

		// Now pull _one_ image from a multi-image archive via the name
		// and index syntax.
		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:@0")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:example.com/empty:latest")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:@1")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:example.com/empty/but:different")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Now check for some errors.
		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:foo.com/does/not/exist:latest")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		expectedError = "Tag \"foo.com/does/not/exist:latest\" not found"
		found, _ = session.ErrorGrepString(expectedError)
		Expect(found).To(Equal(true))

		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("docker-archive:./testdata/image/docker-two-images.tar.xz:@2")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		expectedError = "Invalid source index @2, only 2 manifest items available"
		found, _ = session.ErrorGrepString(expectedError)
		Expect(found).To(Equal(true))
	})

	It("podman pull from oci-archive", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		podmanTest.RestoreArtifact(ALPINE)
		tarfn := filepath.Join(podmanTest.TempDir, "oci-alp.tar")
		session := podmanTest.PodmanNoCache([]string{"save", "--format", "oci-archive", "-o", tarfn, "alpine"})
		session.WaitWithDefaultTimeout()

		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"pull", fmt.Sprintf("oci-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from local directory", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		podmanTest.RestoreArtifact(ALPINE)
		dirpath := filepath.Join(podmanTest.TempDir, "alpine")
		os.MkdirAll(dirpath, os.ModePerm)
		imgPath := fmt.Sprintf("dir:%s", dirpath)

		session := podmanTest.PodmanNoCache([]string{"push", "alpine", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"pull", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContainsTag(filepath.Join("localhost", dirpath), "latest")).To(BeTrue())
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull from local OCI directory", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		podmanTest.RestoreArtifact(ALPINE)
		dirpath := filepath.Join(podmanTest.TempDir, "alpine")
		os.MkdirAll(dirpath, os.ModePerm)
		imgPath := fmt.Sprintf("oci:%s", dirpath)

		session := podmanTest.PodmanNoCache([]string{"push", "alpine", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"pull", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContainsTag(filepath.Join("localhost", dirpath), "latest")).To(BeTrue())
		session = podmanTest.PodmanNoCache([]string{"rmi", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pull check quiet", func() {
		podmanTest.RestoreArtifact(ALPINE)
		setup := podmanTest.PodmanNoCache([]string{"images", ALPINE, "-q", "--no-trunc"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		shortImageId := strings.Split(setup.OutputToString(), ":")[1]

		rmi := podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		pull := podmanTest.PodmanNoCache([]string{"pull", "-q", ALPINE})
		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(Equal(0))

		Expect(pull.OutputToString()).To(ContainSubstring(shortImageId))
	})

	It("podman pull check all tags", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "--all-tags", "k8s.gcr.io/pause"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOuputStartsWith("Pulled Images:")).To(BeTrue())

		session = podmanTest.PodmanNoCache([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 4))
	})

	It("podman pull from docker with nonexist --authfile", func() {
		session := podmanTest.PodmanNoCache([]string{"pull", "--authfile", "/tmp/nonexist", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})
})
