package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/archive"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman push", func() {

	BeforeEach(func() {
		podmanTest.AddImageToRWStore(ALPINE)
	})

	It("podman push to containers/storage", func() {
		SkipIfRemote("Remote push does not support containers-storage transport")
		session := podmanTest.Podman([]string{"push", ALPINE, "containers-storage:busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to dir", func() {
		SkipIfRemote("Remote push does not support dir transport")
		bbdir := filepath.Join(podmanTest.TempDir, "busybox")
		session := podmanTest.Podman([]string{"push", "--remove-signatures", ALPINE,
			fmt.Sprintf("dir:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		bbdir = filepath.Join(podmanTest.TempDir, "busybox")
		session = podmanTest.Podman([]string{"push", "--format", "oci", ALPINE,
			fmt.Sprintf("dir:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to oci with compression-format and compression-level", func() {
		SkipIfRemote("Remote push does not support dir transport")
		bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")

		// Invalid compression format specified, it must fail
		session := podmanTest.Podman([]string{"push", "--compression-format=gzip", "--compression-level=40", ALPINE, fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		output := session.ErrorToString()
		Expect(output).To(ContainSubstring("invalid compression level"))

		session = podmanTest.Podman([]string{"push", "--compression-format=zstd", "--remove-signatures", ALPINE,
			fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		foundZstdFile := false

		blobsDir := filepath.Join(bbdir, "blobs/sha256")

		blobs, err := os.ReadDir(blobsDir)
		Expect(err).ToNot(HaveOccurred())

		for _, f := range blobs {
			blobPath := filepath.Join(blobsDir, f.Name())

			sourceFile, err := os.ReadFile(blobPath)
			Expect(err).ToNot(HaveOccurred())

			compressionType := archive.DetectCompression(sourceFile)
			if compressionType == archive.Zstd {
				foundZstdFile = true
				break
			}
		}
		Expect(foundZstdFile).To(BeTrue(), "found zstd file")
	})

	It("podman push to local registry", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
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

		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(BeEmpty())

		push = podmanTest.Podman([]string{"push", "--compression-format=gzip", "--compression-level=1", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		output := push.ErrorToString()
		Expect(output).To(ContainSubstring("Copying blob "))
		Expect(output).To(ContainSubstring("Copying config "))
		Expect(output).To(ContainSubstring("Writing manifest to image destination"))

		bitSize := 1024
		keyFileName := filepath.Join(podmanTest.TempDir, "key")
		publicKeyFileName, _, err := WriteRSAKeyPair(keyFileName, bitSize)
		Expect(err).ToNot(HaveOccurred())

		if !IsRemote() { // Remote does not support --encryption-key
			push = podmanTest.Podman([]string{"push", "--encryption-key", "jwe:" + publicKeyFileName, "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
			push.WaitWithDefaultTimeout()
			Expect(push).Should(Exit(0))
		}

		// Test --digestfile option
		digestFile := filepath.Join(podmanTest.TempDir, "digestfile.txt")
		push2 := podmanTest.Podman([]string{"push", "--tls-verify=false", "--digestfile=" + digestFile, "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push2.WaitWithDefaultTimeout()
		fi, err := os.Lstat(digestFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(fi.Name()).To(Equal("digestfile.txt"))
		Expect(push2).Should(Exit(0))

		if !IsRemote() { // Remote does not support signing
			By("pushing and pulling with --sign-by-sigstore-private-key")
			// Ideally, this should set SystemContext.RegistriesDirPath, but Podman currently doesn’t
			// expose that as an option. So, for now, modify /etc/directly, and skip testing sigstore if
			// we don’t have permission to do so.
			systemRegistriesDAddition := "/etc/containers/registries.d/podman-test-only-temporary-addition.yaml"
			cmd := exec.Command("cp", "testdata/sigstore-registries.d-fragment.yaml", systemRegistriesDAddition)
			output, err := cmd.CombinedOutput()
			if err != nil {
				GinkgoWriter.Printf("Skipping sigstore tests because /etc/containers/registries.d isn’t writable: %s\n", string(output))
			} else {
				defer func() {
					err := os.Remove(systemRegistriesDAddition)
					Expect(err).ToNot(HaveOccurred())
				}()
				// Generate a signature verification policy file
				policyPath := generatePolicyFile(podmanTest.TempDir)
				defer os.Remove(policyPath)

				// Verify that the policy rejects unsigned images
				push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/sigstore-signed"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(Exit(0))
				Expect(push.ErrorToString()).To(BeEmpty())

				pull := podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5000/sigstore-signed"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).To(ExitWithError())
				Expect(pull.ErrorToString()).To(ContainSubstring("A signature was required, but no signature exists"))

				// Sign an image, and verify it is accepted.
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", "--sign-by-sigstore-private-key", "testdata/sigstore-key.key", "--sign-passphrase-file", "testdata/sigstore-key.key.pass", ALPINE, "localhost:5000/sigstore-signed"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(Exit(0))
				Expect(push.ErrorToString()).To(BeEmpty())

				pull = podmanTest.Podman([]string{"pull", "-q", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5000/sigstore-signed"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).Should(Exit(0))

				By("pushing and pulling with --sign-by-sigstore")
				// Verify that the policy rejects unsigned images
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/sigstore-signed-params"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(Exit(0))
				Expect(push.ErrorToString()).To(BeEmpty())

				pull = podmanTest.Podman([]string{"pull", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5000/sigstore-signed-params"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).To(ExitWithError())
				Expect(pull.ErrorToString()).To(ContainSubstring("A signature was required, but no signature exists"))

				// Sign an image, and verify it is accepted.
				push = podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", "--sign-by-sigstore", "testdata/sigstore-signing-params.yaml", ALPINE, "localhost:5000/sigstore-signed-params"})
				push.WaitWithDefaultTimeout()
				Expect(push).Should(Exit(0))
				Expect(push.ErrorToString()).To(BeEmpty())

				pull = podmanTest.Podman([]string{"pull", "--tls-verify=false", "--signature-policy", policyPath, "localhost:5000/sigstore-signed-params"})
				pull.WaitWithDefaultTimeout()
				Expect(pull).Should(Exit(0))
			}
		}
	})

	It("podman push from local storage with nothing-allowed signature policy", func() {
		SkipIfRemote("Remote push does not support dir transport")
		denyAllPolicy := filepath.Join(INTEGRATION_ROOT, "test/deny.json")

		inspect := podmanTest.Podman([]string{"inspect", "--format={{.ID}}", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		imageID := inspect.OutputToString()

		push := podmanTest.Podman([]string{"push", "--signature-policy", denyAllPolicy, "-q", imageID, "dir:" + filepath.Join(podmanTest.TempDir, imageID)})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(BeEmpty())
	})

	It("podman push to local registry with authorization", func() {
		SkipIfRootless("/etc/containers/certs.d not writable")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		authPath := filepath.Join(podmanTest.TempDir, "auth")
		err = os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		err = os.MkdirAll("/etc/containers/certs.d/localhost:5000", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll("/etc/containers/certs.d/localhost:5000")

		cwd, _ := os.Getwd()
		certPath := filepath.Join(cwd, "../", "certs")

		lock := GetPortLock("5000")
		defer lock.Unlock()
		htpasswd := SystemExec("htpasswd", []string{"-Bbn", "podmantest", "test"})
		htpasswd.WaitWithDefaultTimeout()
		Expect(htpasswd).Should(Exit(0))

		f, err := os.Create(filepath.Join(authPath, "htpasswd"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		_, err = f.WriteString(htpasswd.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		err = f.Sync()
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"logs", "registry"})
		session.WaitWithDefaultTimeout()

		push := podmanTest.Podman([]string{"push", "--tls-verify=true", "--format=v2s2", "--creds=podmantest:test", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())
		Expect(push.ErrorToString()).To(ContainSubstring("x509: certificate signed by unknown authority"))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", "--tls-verify=false", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:5000/ca.crt"})
		Expect(setup).Should(Exit(0))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:wrongpasswd", ALPINE, "localhost:5000/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())
		Expect(push.ErrorToString()).To(ContainSubstring("/credstest: authentication required"))

		if !IsRemote() {
			// remote does not support --cert-dir
			push = podmanTest.Podman([]string{"push", "--tls-verify=true", "--creds=podmantest:test", "--cert-dir=fakedir", ALPINE, "localhost:5000/certdirtest"})
			push.WaitWithDefaultTimeout()
			Expect(push).To(ExitWithError())
			Expect(push.ErrorToString()).To(ContainSubstring("x509: certificate signed by unknown authority"))
		}

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/defaultflags"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		Expect(push.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

		// create and push manifest
		session = podmanTest.Podman([]string{"manifest", "create", "localhost:5000/manifesttest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "push", "--creds=podmantest:test", "--tls-verify=false", "--all", "localhost:5000/manifesttest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Writing manifest list to image destination"))
	})

	It("podman push and encrypt to oci", func() {
		SkipIfRemote("Remote push neither supports oci transport, nor encryption")

		bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")

		bitSize := 1024
		keyFileName := filepath.Join(podmanTest.TempDir, "key")
		publicKeyFileName, _, err := WriteRSAKeyPair(keyFileName, bitSize)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"push", "--encryption-key", "jwe:" + publicKeyFileName, ALPINE, fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to docker-archive", func() {
		SkipIfRemote("Remote push does not support docker-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{"push", ALPINE,
			fmt.Sprintf("docker-archive:%s:latest", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to docker daemon", func() {
		SkipIfRemote("Remote push does not support docker-daemon transport")
		SkipIfRootless("rootless user has no permission to use default docker.sock")
		setup := SystemExec("bash", []string{"-c", "systemctl status docker 2>&1"})

		if setup.LineInOutputContains("Active: inactive") {
			setup = SystemExec("systemctl", []string{"start", "docker"})
			Expect(setup).Should(Exit(0))
			defer func() {
				stop := SystemExec("systemctl", []string{"stop", "docker"})
				Expect(stop).Should(Exit(0))
			}()
		} else if setup.ExitCode() != 0 {
			Skip("Docker is not available")
		}

		session := podmanTest.Podman([]string{"push", ALPINE, "docker-daemon:alpine:podmantest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := SystemExec("docker", []string{"images", "--format", "{{.Repository}}:{{.Tag}}"})
		Expect(check).Should(Exit(0))
		Expect(check.OutputToString()).To(ContainSubstring("alpine:podmantest"))

		clean := SystemExec("docker", []string{"rmi", "alpine:podmantest"})
		Expect(clean).Should(Exit(0))
	})

	It("podman push to oci-archive", func() {
		SkipIfRemote("Remote push does not support oci-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{"push", ALPINE,
			fmt.Sprintf("oci-archive:%s:latest", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to docker-archive no reference", func() {
		SkipIfRemote("Remote push does not support docker-archive transport")
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.Podman([]string{"push", ALPINE,
			fmt.Sprintf("docker-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman push to oci-archive no reference", func() {
		SkipIfRemote("Remote push does not support oci-archive transport")
		ociarc := filepath.Join(podmanTest.TempDir, "alp-oci")
		session := podmanTest.Podman([]string{"push", ALPINE,
			fmt.Sprintf("oci-archive:%s", ociarc)})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

})
