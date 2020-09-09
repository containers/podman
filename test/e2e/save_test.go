package integration

import (
	"os"
	"path/filepath"

	"github.com/containers/podman/v2/pkg/rootless"
	. "github.com/containers/podman/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman save", func() {
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

	It("podman save output flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save oci flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save with stdout", func() {
		Skip("Pipe redirection in ginkgo probably won't work")
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", ALPINE, ">", outfile})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bogus image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, "FOOBAR"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save to directory with oci format", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with v2s2 docker format", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save to directory with docker format and compression", func() {
		if rootless.IsRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.PodmanNoCache([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save bad filename", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save:colon")

		save := podmanTest.PodmanNoCache([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save image with digest reference", func() {
		// pull a digest reference
		session := podmanTest.PodmanNoCache([]string{"pull", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// save a digest reference should exit without error.
		outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
		save := podmanTest.PodmanNoCache([]string{"save", "-o", outfile, ALPINELISTDIGEST})
		save.WaitWithDefaultTimeout()
		Expect(save.ExitCode()).To(Equal(0))
	})

	It("podman save --multi-image-archive (tagged images)", func() {
		multiImageSave(podmanTest, RESTORE_IMAGES)
	})

	It("podman save --multi-image-archive (untagged images)", func() {
		// Refer to images via ID instead of tag.
		session := podmanTest.PodmanNoCache([]string{"images", "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ids := session.OutputToStringArray()

		Expect(len(RESTORE_IMAGES), len(ids))
		multiImageSave(podmanTest, ids)
	})
})

// Create a multi-image archive, remove all images, load it and
// make sure that all images are (again) present.
func multiImageSave(podmanTest *PodmanTestIntegration, images []string) {
	// Create the archive.
	outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
	session := podmanTest.PodmanNoCache(append([]string{"save", "-o", outfile, "--multi-image-archive"}, images...))
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(Equal(0))

	// Remove all images.
	session = podmanTest.PodmanNoCache([]string{"rmi", "-af"})
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(Equal(0))

	// Now load the archive.
	session = podmanTest.PodmanNoCache([]string{"load", "-i", outfile})
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(Equal(0))
	// Grep for each image in the `podman load` output.
	for _, image := range images {
		found, _ := session.GrepString(image)
		Expect(found).Should(BeTrue())
	}

	// Make sure that each image has really been loaded.
	for _, image := range images {
		session = podmanTest.PodmanNoCache([]string{"image", "exists", image})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	}
}
