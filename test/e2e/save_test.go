package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentSpecReport()
		processTestResult(f)

	})

	It("podman save output flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save signature-policy flag", func() {
		SkipIfRemote("--signature-policy N/A for remote")
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "--signature-policy", "/etc/containers/policy.json", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save oci flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save with stdout", func() {
		Skip("Pipe redirection in ginkgo probably won't work")
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", ALPINE, ">", outfile})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save bogus image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-o", outfile, "FOOBAR"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save to directory with oci format", func() {
		if isRootless() {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save to directory with v2s2 docker format", func() {
		if isRootless() {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save to directory with docker format and compression", func() {
		if isRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save to directory with --compress but not use docker-dir and oci-dir", func() {
		if isRootless() && podmanTest.RemoteTest {
			Skip("Requires a fix in containers image for chown/lchown")
		}
		outdir := filepath.Join(podmanTest.TempDir, "save")

		save := podmanTest.Podman([]string{"save", "--compress", "--format", "docker-archive", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		// should not be 0
		Expect(save).To(ExitWithError())

		save = podmanTest.Podman([]string{"save", "--compress", "--format", "oci-archive", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		// should not be 0
		Expect(save).To(ExitWithError())

	})

	It("podman save bad filename", func() {
		outdir := filepath.Join(podmanTest.TempDir, "save:colon")

		save := podmanTest.Podman([]string{"save", "--compress", "--format", "docker-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError())
	})

	It("podman save remove signature", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		SkipIfRootless("FIXME: Need get in rootless push sign")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		tempGNUPGHOME := filepath.Join(podmanTest.TempDir, "tmpGPG")
		err := os.Mkdir(tempGNUPGHOME, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		origGNUPGHOME := os.Getenv("GNUPGHOME")
		err = os.Setenv("GNUPGHOME", tempGNUPGHOME)
		Expect(err).ToNot(HaveOccurred())
		defer os.Setenv("GNUPGHOME", origGNUPGHOME)

		port := 5000
		portlock := GetPortLock(strconv.Itoa(port))
		defer portlock.Unlock()

		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", strings.Join([]string{strconv.Itoa(port), strconv.Itoa(port)}, ":"), REGISTRY_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		cmd := exec.Command("gpg", "--import", "sign/secret-key.asc")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())

		defaultYaml := filepath.Join(podmanTest.TempDir, "default.yaml")
		cmd = exec.Command("cp", "/etc/containers/registries.d/default.yaml", defaultYaml)
		if err = cmd.Run(); err != nil {
			Skip("no signature store to verify")
		}
		defer func() {
			cmd = exec.Command("cp", defaultYaml, "/etc/containers/registries.d/default.yaml")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
		}()

		keyPath := filepath.Join(podmanTest.TempDir, "key.gpg")
		cmd = exec.Command("cp", "sign/key.gpg", keyPath)
		Expect(cmd.Run()).To(Succeed())
		defer os.Remove(keyPath)

		sigstore := `
default-docker:
  sigstore: file:///var/lib/containers/sigstore
  sigstore-staging: file:///var/lib/containers/sigstore
`
		Expect(os.WriteFile("/etc/containers/registries.d/default.yaml", []byte(sigstore), 0755)).To(Succeed())

		session = podmanTest.Podman([]string{"tag", ALPINE, "localhost:5000/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", "--tls-verify=false", "--sign-by", "foo@bar.com", "localhost:5000/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", ALPINE, "localhost:5000/alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !IsRemote() {
			// Generate a signature verification policy file
			policyPath := generatePolicyFile(podmanTest.TempDir)
			defer os.Remove(policyPath)

			session = podmanTest.Podman([]string{"pull", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5000/alpine"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
			save := podmanTest.Podman([]string{"save", "remove-signatures=true", "-o", outfile, "localhost:5000/alpine"})
			save.WaitWithDefaultTimeout()
			Expect(save).To(ExitWithError())
		}
	})

	It("podman save image with digest reference", func() {
		// pull a digest reference
		session := podmanTest.Podman([]string{"pull", ALPINELISTDIGEST})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// save a digest reference should exit without error.
		outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
		save := podmanTest.Podman([]string{"save", "-o", outfile, ALPINELISTDIGEST})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(Exit(0))
	})

	It("podman save --multi-image-archive (tagged images)", func() {
		multiImageSave(podmanTest, RESTORE_IMAGES)
	})

	It("podman save --multi-image-archive (untagged images)", func() {
		// #14468: to make execution time more predictable, save at
		// most three images and sort them by size.
		session := podmanTest.Podman([]string{"images", "--sort", "size", "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		ids := session.OutputToStringArray()

		Expect(len(ids)).To(BeNumerically(">", 1), "We need to have *some* images to save")
		if len(ids) > 3 {
			ids = ids[:3]
		}
		multiImageSave(podmanTest, ids)
	})
})

// Create a multi-image archive, remove all images, load it and
// make sure that all images are (again) present.
func multiImageSave(podmanTest *PodmanTestIntegration, images []string) {
	// Create the archive.
	outfile := filepath.Join(podmanTest.TempDir, "temp.tar")
	session := podmanTest.Podman(append([]string{"save", "-o", outfile, "--multi-image-archive"}, images...))
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))

	// Remove all images.
	session = podmanTest.Podman([]string{"rmi", "-af"})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))

	// Now load the archive.
	session = podmanTest.Podman([]string{"load", "-i", outfile})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(Exit(0))
	// Grep for each image in the `podman load` output.
	for _, image := range images {
		Expect(session.OutputToString()).To(ContainSubstring(image))
	}

	// Make sure that each image has really been loaded.
	for _, image := range images {
		session = podmanTest.Podman([]string{"image", "exists", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	}
}
