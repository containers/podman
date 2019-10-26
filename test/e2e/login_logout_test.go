// +build !remoteclient

package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman login and logout", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
		authPath   string
		certPath   string
		port       int
		server     string
		testImg    string
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.RestoreAllArtifacts()

		authPath = filepath.Join(podmanTest.TempDir, "auth")
		os.Mkdir(authPath, os.ModePerm)

		if IsCommandAvailable("getenforce") {
			ge := SystemExec("getenforce", []string{})
			ge.WaitWithDefaultTimeout()
			if ge.OutputToString() == "Enforcing" {
				se := SystemExec("setenforce", []string{"0"})
				se.WaitWithDefaultTimeout()
				if se.ExitCode() != 0 {
					Skip("Can not disable selinux, this may cause problem for reading cert files inside container.")
				}
				defer SystemExec("setenforce", []string{"1"})
			}
		}

		session := podmanTest.Podman([]string{"run", "--entrypoint", "htpasswd", "registry:2", "-Bbn", "podmantest", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		f, _ := os.Create(filepath.Join(authPath, "htpasswd"))
		defer f.Close()

		f.WriteString(session.OutputToString())
		f.Sync()
		port = 4999 + config.GinkgoConfig.ParallelNode
		server = strings.Join([]string{"localhost", strconv.Itoa(port)}, ":")
		testImg = strings.Join([]string{server, "test-apline"}, "/")

		os.MkdirAll(filepath.Join("/etc/containers/certs.d", server), os.ModePerm)

		cwd, _ := os.Getwd()
		certPath = filepath.Join(cwd, "../", "certs")

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), filepath.Join("/etc/containers/certs.d", server, "ca.crt")})
		setup.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "-p", strings.Join([]string{strconv.Itoa(port), strconv.Itoa(port)}, ":"),
			"-e", strings.Join([]string{"REGISTRY_HTTP_ADDR=0.0.0.0", strconv.Itoa(port)}, ":"), "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", "registry:2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		os.RemoveAll(authPath)
		os.RemoveAll(filepath.Join("/etc/containers/certs.d", server))
	})

	It("podman login and logout", func() {
		session := podmanTest.Podman([]string{"login", "-u", "podmantest", "-p", "test", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman login and logout with flag --authfile", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		authInfo, _ := ioutil.ReadFile(authFile)
		var info map[string]interface{}
		json.Unmarshal(authInfo, &info)
		fmt.Println(info)

		// push should fail with nonexist authfile
		session = podmanTest.Podman([]string{"push", "--authfile", "/tmp/nonexist", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"push", "--authfile", authFile, ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"run", "--authfile", authFile, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// logout should fail with nonexist authfile
		session = podmanTest.Podman([]string{"logout", "--authfile", "/tmp/nonexist", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		session = podmanTest.Podman([]string{"logout", "--authfile", authFile, server})
	})

	It("podman login and logout with --tls-verify", func() {
		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--tls-verify=false", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman login and logout with --cert-dir", func() {
		certDir := filepath.Join(podmanTest.TempDir, "certs")
		os.MkdirAll(certDir, os.ModePerm)

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), filepath.Join(certDir, "ca.crt")})
		setup.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--cert-dir", certDir, server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman login and logout with multi registry", func() {
		os.MkdirAll("/etc/containers/certs.d/localhost:9001", os.ModePerm)

		cwd, _ := os.Getwd()
		certPath = filepath.Join(cwd, "../", "certs")

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), "/etc/containers/certs.d/localhost:9001/ca.crt"})
		setup.WaitWithDefaultTimeout()
		defer os.RemoveAll("/etc/containers/certs.d/localhost:9001")

		session := podmanTest.Podman([]string{"run", "-d", "-p", "9001:9001", "-e", "REGISTRY_HTTP_ADDR=0.0.0.0:9001", "--name", "registry1", "-v",
			strings.Join([]string{authPath, "/auth"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", "registry:2"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry1", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"logout", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})
})
