package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman pull multiple images with/without tag/digest", func() {
		session := podmanTest.Podman([]string{"pull", "busybox:musl", "alpine", "alpine:latest", "quay.io/libpod/cirros", "quay.io/libpod/testdigest_v2s2@sha256:755f4d90b3716e2bf57060d249e2cd61c9ac089b1233465c5c2cb2d7ee550fdb"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pull", "busybox:latest", "docker.io/library/ibetthisdoesnotexistfr:random", "alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError := "initializing source docker://ibetthisdoesnotexistfr:random"
		found, _ := session.ErrorGrepString(expectedError)
		Expect(found).To(BeTrue())

		session = podmanTest.Podman([]string{"rmi", "busybox:musl", "alpine", "quay.io/libpod/cirros", "testdigest_v2s2@sha256:755f4d90b3716e2bf57060d249e2cd61c9ac089b1233465c5c2cb2d7ee550fdb"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull bogus image", func() {
		session := podmanTest.Podman([]string{"pull", "quay.io/ibetthis/doesntexistthere:foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pull with tag --quiet", func() {
		session := podmanTest.Podman([]string{"pull", "-q", "quay.io/libpod/testdigest_v2s2:20200210"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		quietOutput := session.OutputToString()

		session = podmanTest.Podman([]string{"inspect", "testdigest_v2s2:20200210", "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(quietOutput))

		session = podmanTest.Podman([]string{"rmi", "testdigest_v2s2:20200210"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull without tag", func() {
		session := podmanTest.Podman([]string{"pull", "quay.io/libpod/testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", "testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull by digest", func() {
		session := podmanTest.Podman([]string{"pull", "quay.io/libpod/testdigest_v2s2@sha256:755f4d90b3716e2bf57060d249e2cd61c9ac089b1233465c5c2cb2d7ee550fdb"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Without a tag/digest the input is normalized with the "latest" tag, see #11964
		session = podmanTest.Podman([]string{"rmi", "testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		session = podmanTest.Podman([]string{"rmi", "testdigest_v2s2@sha256:755f4d90b3716e2bf57060d249e2cd61c9ac089b1233465c5c2cb2d7ee550fdb"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull check all tags", func() {
		session := podmanTest.Podman([]string{"pull", "--all-tags", "quay.io/libpod/testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">=", 2), "Expected at least two images")

		session = podmanTest.Podman([]string{"pull", "-a", "quay.io/libpod/testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">=", 2), "Expected at least two images")
	})

	It("podman pull from docker with nonexistent --authfile", func() {
		session := podmanTest.Podman([]string{"pull", "--authfile", "/tmp/nonexistent", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman pull by digest (image list)", func() {
		session := podmanTest.Podman([]string{"pull", "--arch=arm64", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the digest of the list
		session = podmanTest.Podman([]string{"rmi", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull by instance digest (image list)", func() {
		session := podmanTest.Podman([]string{"pull", "--arch=arm64", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(Not(ContainSubstring(ALPINELISTDIGEST)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(HavePrefix("[]"))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(Not(ContainSubstring(ALPINELISTDIGEST)))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the digest of the instance
		session = podmanTest.Podman([]string{"rmi", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull by tag (image list)", func() {
		session := podmanTest.Podman([]string{"pull", "--arch=arm64", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// inspect using the tag we used for pulling
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the tag we used for pulling
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the digest of the list
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the digest of the arch-specific image's manifest
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64DIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoTags}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTTAG))
		// inspect using the image ID
		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RepoDigests}}", ALPINEARM64ID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINELISTDIGEST))
		Expect(string(session.Out.Contents())).To(ContainSubstring(ALPINEARM64DIGEST))
		// remove using the tag
		session = podmanTest.Podman([]string{"rmi", ALPINELISTTAG})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull from docker-archive", func() {
		SkipIfRemote("podman-remote does not support pulling from docker-archive")

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		tarfn := filepath.Join(podmanTest.TempDir, "cirros.tar")
		session := podmanTest.Podman([]string{"save", "-o", tarfn, "cirros"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"pull", fmt.Sprintf("docker-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Pulling a multi-image archive without further specifying
		// which image _must_ error out. Pulling is restricted to one
		// image.
		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError := "Unexpected tar manifest.json: expected 1 item, got 2"
		found, _ := session.ErrorGrepString(expectedError)
		Expect(found).To(BeTrue())

		// Now pull _one_ image from a multi-image archive via the name
		// and index syntax.
		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:@0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:example.com/empty:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:@1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:example.com/empty/but:different"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Now check for some errors.
		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:foo.com/does/not/exist:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError = "Tag \"foo.com/does/not/exist:latest\" not found"
		found, _ = session.ErrorGrepString(expectedError)
		Expect(found).To(BeTrue())

		session = podmanTest.Podman([]string{"pull", "docker-archive:./testdata/docker-two-images.tar.xz:@2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError = "Invalid source index @2, only 2 manifest items available"
		found, _ = session.ErrorGrepString(expectedError)
		Expect(found).To(BeTrue())
	})

	It("podman pull from oci-archive", func() {
		SkipIfRemote("podman-remote does not support pulling from oci-archive")

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		tarfn := filepath.Join(podmanTest.TempDir, "oci-cirrus.tar")
		session := podmanTest.Podman([]string{"save", "--format", "oci-archive", "-o", tarfn, "cirros"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"pull", fmt.Sprintf("oci-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman pull from local directory", func() {
		SkipIfRemote("podman-remote does not support pulling from local directory")

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dirpath := filepath.Join(podmanTest.TempDir, "cirros")
		err = os.MkdirAll(dirpath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		imgPath := fmt.Sprintf("dir:%s", dirpath)

		session := podmanTest.Podman([]string{"push", "cirros", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"run", imgPath, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Note that reference is not preserved in dir.
		session = podmanTest.Podman([]string{"image", "exists", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman pull from local OCI directory", func() {
		SkipIfRemote("podman-remote does not support pulling from OCI directory")

		podmanTest.AddImageToRWStore(CIRROS_IMAGE)
		dirpath := filepath.Join(podmanTest.TempDir, "cirros")
		err = os.MkdirAll(dirpath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		imgPath := fmt.Sprintf("oci:%s", dirpath)

		session := podmanTest.Podman([]string{"push", "cirros", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"rmi", "cirros"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"pull", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContainsTag(filepath.Join("localhost", dirpath), "latest")).To(BeTrue())
	})

	It("podman pull + inspect from unqualified-search registry", func() {
		// Regression test for #6381:
		// Make sure that `pull shortname` and `inspect shortname`
		// refer to the same image.

		// We already tested pulling, so we can save some energy and
		// just restore local artifacts and tag them.
		err := podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		err = podmanTest.RestoreArtifact(BB)
		Expect(err).ToNot(HaveOccurred())

		// What we want is at least two images which have the same name
		// and are prefixed with two different unqualified-search
		// registries from ../registries.conf.
		//
		// A `podman inspect $name` must yield the one from the _first_
		// matching registry in the registries.conf.
		getID := func(image string) string {
			setup := podmanTest.Podman([]string{"image", "inspect", image})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))

			data := setup.InspectImageJSON() // returns []inspect.ImageData
			Expect(data).To(HaveLen(1))
			return data[0].ID
		}

		untag := func(image string) {
			setup := podmanTest.Podman([]string{"untag", image})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))

			setup = podmanTest.Podman([]string{"image", "inspect", image})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))

			data := setup.InspectImageJSON() // returns []inspect.ImageData
			Expect(data).To(HaveLen(1))
			Expect(data[0].RepoTags).To(BeEmpty())
		}

		tag := func(image, tag string) {
			setup := podmanTest.Podman([]string{"tag", image, tag})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))
			setup = podmanTest.Podman([]string{"image", "exists", tag})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))
		}

		image1 := getID(ALPINE)
		image2 := getID(BB)

		// $ head -n2 ../registries.conf
		// [registries.search]
		// registries = ['docker.io', 'quay.io', 'registry.fedoraproject.org']
		registries := []string{"docker.io", "quay.io", "registry.fedoraproject.org"}
		name := "foo/test:tag"
		tests := []struct {
			// tag1 has precedence (see list above) over tag2 when
			// doing an inspect on "test:tag".
			tag1, tag2 string
		}{
			{
				fmt.Sprintf("%s/%s", registries[0], name),
				fmt.Sprintf("%s/%s", registries[1], name),
			},
			{
				fmt.Sprintf("%s/%s", registries[0], name),
				fmt.Sprintf("%s/%s", registries[2], name),
			},
			{
				fmt.Sprintf("%s/%s", registries[1], name),
				fmt.Sprintf("%s/%s", registries[2], name),
			},
		}

		for _, t := range tests {
			// 1) untag both images
			// 2) tag them according to `t`
			// 3) make sure that an inspect of `name` returns `image1` with `tag1`
			untag(image1)
			untag(image2)
			tag(image1, t.tag1)
			tag(image2, t.tag2)

			setup := podmanTest.Podman([]string{"image", "inspect", name})
			setup.WaitWithDefaultTimeout()
			Expect(setup).Should(Exit(0))

			data := setup.InspectImageJSON() // returns []inspect.ImageData
			Expect(data).To(HaveLen(1))
			Expect(data[0].RepoTags).To(HaveLen(1))
			Expect(data[0].RepoTags[0]).To(Equal(t.tag1))
			Expect(data[0]).To(HaveField("ID", image1))
		}
	})

	It("podman pull --platform", func() {
		session := podmanTest.Podman([]string{"pull", "--platform=linux/bogus", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError := "no image found in manifest list for architecture bogus"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"pull", "--platform=linux/arm64", "--os", "windows", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError = "--platform option can not be specified with --arch or --os"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"pull", "-q", "--platform=linux/arm64", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		setup := podmanTest.Podman([]string{"image", "inspect", session.OutputToString()})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		data := setup.InspectImageJSON() // returns []inspect.ImageData
		Expect(data).To(HaveLen(1))
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", "arm64"))
	})

	It("podman pull --arch", func() {
		session := podmanTest.Podman([]string{"pull", "--arch=bogus", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError := "no image found in manifest list for architecture bogus"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"pull", "--arch=arm64", "--os", "windows", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		expectedError = "no image found in manifest list for architecture"
		Expect(session.ErrorToString()).To(ContainSubstring(expectedError))

		session = podmanTest.Podman([]string{"pull", "-q", "--arch=arm64", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		setup := podmanTest.Podman([]string{"image", "inspect", session.OutputToString()})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(Exit(0))

		data := setup.InspectImageJSON() // returns []inspect.ImageData
		Expect(data).To(HaveLen(1))
		Expect(data[0]).To(HaveField("Os", runtime.GOOS))
		Expect(data[0]).To(HaveField("Architecture", "arm64"))
	})

	It("podman pull progress", func() {
		session := podmanTest.Podman([]string{"pull", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.ErrorToString()
		Expect(output).To(ContainSubstring("Getting image source signatures"))
		Expect(output).To(ContainSubstring("Copying blob "))

		session = podmanTest.Podman([]string{"pull", "-q", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(BeEmpty())
	})

	Describe("podman pull and decrypt", func() {

		decryptionTestHelper := func(imgPath string) *PodmanSessionIntegration {
			bitSize := 1024
			keyFileName := filepath.Join(podmanTest.TempDir, "key")
			publicKeyFileName, privateKeyFileName, err := WriteRSAKeyPair(keyFileName, bitSize)
			Expect(err).ToNot(HaveOccurred())

			wrongKeyFileName := filepath.Join(podmanTest.TempDir, "wrong_key")
			_, wrongPrivateKeyFileName, err := WriteRSAKeyPair(wrongKeyFileName, bitSize)
			Expect(err).ToNot(HaveOccurred())

			session := podmanTest.Podman([]string{"push", "--encryption-key", "jwe:" + publicKeyFileName, "--tls-verify=false", "--remove-signatures", ALPINE, imgPath})
			session.WaitWithDefaultTimeout()

			session = podmanTest.Podman([]string{"rmi", ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			// Pulling encrypted image without key should fail
			session = podmanTest.Podman([]string{"pull", imgPath})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(125))

			// Pulling encrypted image with wrong key should fail
			session = podmanTest.Podman([]string{"pull", "--decryption-key", wrongPrivateKeyFileName, imgPath})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(125))

			// Pulling encrypted image with correct key should pass
			session = podmanTest.Podman([]string{"pull", "--decryption-key", privateKeyFileName, imgPath})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			session = podmanTest.Podman([]string{"images"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			return session
		}

		It("From oci", func() {
			SkipIfRemote("Remote pull neither supports oci transport, nor decryption")

			podmanTest.AddImageToRWStore(ALPINE)

			bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")
			imgPath := fmt.Sprintf("oci:%s", bbdir)

			session := decryptionTestHelper(imgPath)

			Expect(session.LineInOutputContainsTag(filepath.Join("localhost", bbdir), "latest")).To(BeTrue())
		})

		It("From local registry", func() {
			SkipIfRemote("Remote pull does not support decryption")

			if podmanTest.Host.Arch == "ppc64le" {
				Skip("No registry image for ppc64le")
			}

			podmanTest.AddImageToRWStore(ALPINE)

			if isRootless() {
				err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
				Expect(err).ToNot(HaveOccurred())
			}
			lock := GetPortLock("5000")
			defer lock.Unlock()
			session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
				Skip("Cannot start docker registry.")
			}

			imgPath := "localhost:5000/my-alpine"

			session = decryptionTestHelper(imgPath)

			Expect(session.LineInOutputContainsTag(imgPath, "latest")).To(BeTrue())
		})
	})

})
