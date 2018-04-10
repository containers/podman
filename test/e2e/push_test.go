package integration

import (
	"os"
	"path/filepath"
	"strings"

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
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"rmi", "busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to dir", func() {
		session := podmanTest.Podman([]string{"push", "--remove-signatures", ALPINE, "dir:/tmp/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		clean := podmanTest.SystemExec("rm", []string{"-fr", "/tmp/busybox"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to local registry", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", "docker.io/library/registry:2", "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
	})

	It("podman push to local registry with authorization", func() {
		authPath := filepath.Join(podmanTest.TempDir, "auth")
		os.Mkdir(authPath, os.ModePerm)
		os.MkdirAll("/etc/containers/certs.d/localhost:5000", os.ModePerm)
		debug := podmanTest.SystemExec("ls", []string{"-l", podmanTest.TempDir})
		debug.WaitWithDefaultTimeout()

		cwd, _ := os.Getwd()
		certPath := filepath.Join(cwd, "../", "certs")

		setup := podmanTest.SystemExec("getenforce", []string{})
		setup.WaitWithDefaultTimeout()
		if setup.OutputToString() == "Enforcing" {

			setup = podmanTest.SystemExec("setenforce", []string{"0"})
			setup.WaitWithDefaultTimeout()

			defer podmanTest.SystemExec("setenforce", []string{"1"})
		}

		session := podmanTest.Podman([]string{"run", "--entrypoint", "htpasswd", "registry:2", "-Bbn", "podmantest", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		f, _ := os.Create(filepath.Join(authPath, "htpasswd"))
		defer f.Close()

		f.WriteString(session.OutputToString())
		f.Sync()
		debug = podmanTest.SystemExec("cat", []string{filepath.Join(authPath, "htpasswd")})
		debug.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", "registry:2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		session = podmanTest.Podman([]string{"logs", "registry"})
		session.WaitWithDefaultTimeout()

		push := podmanTest.Podman([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Not(Equal(0)))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", "--tls-verify=false", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		setup = podmanTest.SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:5000/ca.crt"})
		setup.WaitWithDefaultTimeout()
		defer os.RemoveAll("/etc/containers/certs.d/localhost:5000")

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:wrongpasswd", ALPINE, "localhost:5000/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Not(Equal(0)))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", "--cert-dir=fakedir", ALPINE, "localhost:5000/certdirtest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Not(Equal(0)))

		push = podmanTest.Podman([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/defaultflags"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
	})

	It("podman push to docker-archive", func() {
		session := podmanTest.Podman([]string{"push", ALPINE, "docker-archive:/tmp/alp:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		clean := podmanTest.SystemExec("rm", []string{"/tmp/alp"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to docker daemon", func() {
		setup := podmanTest.SystemExec("bash", []string{"-c", "systemctl status docker 2>&1"})
		setup.WaitWithDefaultTimeout()

		if setup.LineInOuputContains("Active: inactive") {
			setup = podmanTest.SystemExec("systemctl", []string{"start", "docker"})
			setup.WaitWithDefaultTimeout()

			defer podmanTest.SystemExec("systemctl", []string{"stop", "docker"})
		} else if setup.ExitCode() != 0 {
			Skip("Docker is not avaiable")
		}

		session := podmanTest.Podman([]string{"push", ALPINE, "docker-daemon:alpine:podmantest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := podmanTest.SystemExec("docker", []string{"images", "--format", "{{.Repository}}:{{.Tag}}"})
		check.WaitWithDefaultTimeout()
		Expect(check.ExitCode()).To(Equal(0))
		Expect(check.OutputToString()).To(ContainSubstring("alpine:podmantest"))

		clean := podmanTest.SystemExec("docker", []string{"rmi", "alpine:podmantest"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to oci-archive", func() {
		session := podmanTest.Podman([]string{"push", ALPINE, "oci-archive:/tmp/alp.tar:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		clean := podmanTest.SystemExec("rm", []string{"/tmp/alp.tar"})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to local ostree", func() {
		setup := podmanTest.SystemExec("which", []string{"ostree"})
		setup.WaitWithDefaultTimeout()

		if setup.ExitCode() != 0 {
			Skip("ostree is not installed")
		}

		ostreePath := filepath.Join(podmanTest.TempDir, "ostree/repo")
		os.MkdirAll(ostreePath, os.ModePerm)

		setup = podmanTest.SystemExec("ostree", []string{strings.Join([]string{"--repo=", ostreePath}, ""), "init"})
		setup.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"push", ALPINE, strings.Join([]string{"ostree:alp@", ostreePath}, "")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		clean := podmanTest.SystemExec("rm", []string{"-rf", ostreePath})
		clean.WaitWithDefaultTimeout()
		Expect(clean.ExitCode()).To(Equal(0))
	})

})
