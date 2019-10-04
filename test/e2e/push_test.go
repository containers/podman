// +build !remoteclient

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/rootless"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman push to containers/storage", func() {
		session := podmanTest.PodmanNoCache([]string{"push", ALPINE, "containers-storage:busybox:test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.PodmanNoCache([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to dir", func() {
		bbdir := filepath.Join(podmanTest.TempDir, "busybox")
		session := podmanTest.PodmanNoCache([]string{"push", "--remove-signatures", ALPINE,
			fmt.Sprintf("dir:%s", bbdir)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to local registry", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if rootless.IsRootless() {
			podmanTest.RestoreArtifact(registry)
		}
		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.PodmanNoCache([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		push := podmanTest.PodmanNoCache([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// Test --digestfile option
		push2 := podmanTest.PodmanNoCache([]string{"push", "--tls-verify=false", "--digestfile=/tmp/digestfile.txt", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push2.WaitWithDefaultTimeout()
		fi, err := os.Lstat("/tmp/digestfile.txt")
		Expect(err).To(BeNil())
		Expect(fi.Name()).To(Equal("digestfile.txt"))
		Expect(push2.ExitCode()).To(Equal(0))
	})

	It("podman push to local registry with authorization", func() {
		SkipIfRootless()
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
			Expect(ge.ExitCode()).To(Equal(0))
			if ge.OutputToString() == "Enforcing" {
				se := SystemExec("setenforce", []string{"0"})
				Expect(se.ExitCode()).To(Equal(0))
				defer func() {
					se2 := SystemExec("setenforce", []string{"1"})
					Expect(se2.ExitCode()).To(Equal(0))
				}()
			}
		}
		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.PodmanNoCache([]string{"run", "--entrypoint", "htpasswd", registry, "-Bbn", "podmantest", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		f, _ := os.Create(filepath.Join(authPath, "htpasswd"))
		defer f.Close()

		f.WriteString(session.OutputToString())
		f.Sync()

		session = podmanTest.PodmanNoCache([]string{"run", "-d", "-p", "5000:5000", "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", registry})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		session = podmanTest.PodmanNoCache([]string{"logs", "registry"})
		session.WaitWithDefaultTimeout()

		push := podmanTest.PodmanNoCache([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())

		push = podmanTest.PodmanNoCache([]string{"push", "--creds=podmantest:test", "--tls-verify=false", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:5000/ca.crt"})
		Expect(setup.ExitCode()).To(Equal(0))

		push = podmanTest.PodmanNoCache([]string{"push", "--creds=podmantest:wrongpasswd", ALPINE, "localhost:5000/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())

		push = podmanTest.PodmanNoCache([]string{"push", "--creds=podmantest:test", "--cert-dir=fakedir", ALPINE, "localhost:5000/certdirtest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())

		push = podmanTest.PodmanNoCache([]string{"push", "--creds=podmantest:test", ALPINE, "localhost:5000/defaultflags"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
	})

	It("podman push to docker-archive", func() {
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.PodmanNoCache([]string{"push", ALPINE,
			fmt.Sprintf("docker-archive:%s:latest", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to docker daemon", func() {
		setup := SystemExec("bash", []string{"-c", "systemctl status docker 2>&1"})

		if setup.LineInOutputContains("Active: inactive") {
			setup = SystemExec("systemctl", []string{"start", "docker"})
			defer func() {
				stop := SystemExec("systemctl", []string{"stop", "docker"})
				Expect(stop.ExitCode()).To(Equal(0))
			}()
		} else if setup.ExitCode() != 0 {
			Skip("Docker is not available")
		}

		session := podmanTest.PodmanNoCache([]string{"push", ALPINE, "docker-daemon:alpine:podmantest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		check := SystemExec("docker", []string{"images", "--format", "{{.Repository}}:{{.Tag}}"})
		Expect(check.ExitCode()).To(Equal(0))
		Expect(check.OutputToString()).To(ContainSubstring("alpine:podmantest"))

		clean := SystemExec("docker", []string{"rmi", "alpine:podmantest"})
		Expect(clean.ExitCode()).To(Equal(0))
	})

	It("podman push to oci-archive", func() {
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.PodmanNoCache([]string{"push", ALPINE,
			fmt.Sprintf("oci-archive:%s:latest", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to local ostree", func() {
		if !IsCommandAvailable("ostree") {
			Skip("ostree is not installed")
		}

		ostreePath := filepath.Join(podmanTest.TempDir, "ostree/repo")
		os.MkdirAll(ostreePath, os.ModePerm)

		setup := SystemExec("ostree", []string{strings.Join([]string{"--repo=", ostreePath}, ""), "init"})
		Expect(setup.ExitCode()).To(Equal(0))

		session := podmanTest.PodmanNoCache([]string{"push", ALPINE, strings.Join([]string{"ostree:alp@", ostreePath}, "")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman push to docker-archive no reference", func() {
		tarfn := filepath.Join(podmanTest.TempDir, "alp.tar")
		session := podmanTest.PodmanNoCache([]string{"push", ALPINE,
			fmt.Sprintf("docker-archive:%s", tarfn)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman push to oci-archive no reference", func() {
		ociarc := filepath.Join(podmanTest.TempDir, "alp-oci")
		session := podmanTest.PodmanNoCache([]string{"push", ALPINE,
			fmt.Sprintf("oci-archive:%s", ociarc)})

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

})
