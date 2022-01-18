package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/pkg/rootless"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/archive"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman push", func() {
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
		podmanTest.AddImageToRWStore(ALPINE)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

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

	It("podman push to oci with compression-format", func() {
		SkipIfRemote("Remote push does not support dir transport")
		bbdir := filepath.Join(podmanTest.TempDir, "busybox-oci")
		session := podmanTest.Podman([]string{"push", "--compression-format=zstd", "--remove-signatures", ALPINE,
			fmt.Sprintf("oci:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		foundZstdFile := false

		blobsDir := filepath.Join(bbdir, "blobs/sha256")

		blobs, err := ioutil.ReadDir(blobsDir)
		Expect(err).To(BeNil())

		for _, f := range blobs {
			blobPath := filepath.Join(blobsDir, f.Name())

			sourceFile, err := ioutil.ReadFile(blobPath)
			Expect(err).To(BeNil())

			compressionType := archive.DetectCompression(sourceFile)
			if compressionType == archive.Zstd {
				foundZstdFile = true
				break
			}
		}
		Expect(foundZstdFile).To(BeTrue())
	})

	It("podman push to local registry", func() {
		SkipIfRemote("Remote does not support --digestfile or --remove-signatures")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if rootless.IsRootless() {
			podmanTest.RestoreArtifact(registry)
		}
		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))

		// Test --digestfile option
		push2 := podmanTest.Podman([]string{"push", "--tls-verify=false", "--digestfile=/tmp/digestfile.txt", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push2.WaitWithDefaultTimeout()
		fi, err := os.Lstat("/tmp/digestfile.txt")
		Expect(err).To(BeNil())
		Expect(fi.Name()).To(Equal("digestfile.txt"))
		Expect(push2).Should(Exit(0))
	})

	It("podman push to local registry with authorization", func() {
		SkipIfRootless("volume-mounting a certs.d file N/A over remote")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		authPath := filepath.Join(podmanTest.TempDir, "auth")
		os.Mkdir(authPath, os.ModePerm)
		os.MkdirAll("/etc/containers/certs.d/localhost:5000", os.ModePerm)
		defer os.RemoveAll("/etc/containers/certs.d/localhost:5000")

		cwd, _ := os.Getwd()
		certPath := filepath.Join(cwd, "../", "certs")

		if IsCommandAvailable("getenforce") {
			ge := SystemExec("getenforce", []string{})
			Expect(ge).Should(Exit(0))
			if ge.OutputToString() == "Enforcing" {
				se := SystemExec("setenforce", []string{"0"})
				Expect(se).Should(Exit(0))
				defer func() {
					se2 := SystemExec("setenforce", []string{"1"})
					Expect(se2).Should(Exit(0))
				}()
			}
		}
		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "--entrypoint", "htpasswd", registry, "-Bbn", "podmantest", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		f, _ := os.Create(filepath.Join(authPath, "htpasswd"))
		defer f.Close()

		f.WriteString(session.OutputToString())
		f.Sync()

		session = podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", registry})
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

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", "--tls-verify=false", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:5000/ca.crt"})
		Expect(setup).Should(Exit(0))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:wrongpasswd", ALPINE, "localhost:5000/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())

		if !IsRemote() {
			// remote does not support --cert-dir
			push = podmanTest.Podman([]string{"push", "--tls-verify=true", "--creds=podmantest:test", "--cert-dir=fakedir", ALPINE, "localhost:5000/certdirtest"})
			push.WaitWithDefaultTimeout()
			Expect(push).To(ExitWithError())
		}

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/defaultflags"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
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
		setup := SystemExec("bash", []string{"-c", "systemctl status docker 2>&1"})

		if setup.LineInOutputContains("Active: inactive") {
			setup = SystemExec("systemctl", []string{"start", "docker"})
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
