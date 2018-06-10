package integration

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman search", func() {
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

	It("podman search", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search registry flag", func() {
		search := podmanTest.Podman([]string{"search", "--registry", "registry.fedoraproject.org", "fedora-minimal"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.LineInOutputContains("fedoraproject.org/fedora-minimal")).To(BeTrue())
	})

	It("podman search format flag", func() {
		search := podmanTest.Podman([]string{"search", "--format", "table {{.Index}} {{.Name}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search no-trunc flag", func() {
		search := podmanTest.Podman([]string{"search", "--no-trunc", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
		Expect(search.LineInOutputContains("...")).To(BeFalse())
	})

	It("podman search limit flag", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "3", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(4))
	})

	It("podman search with filter stars", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "stars=10", "--format", "{{.Stars}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(strconv.Atoi(output[i])).To(BeNumerically(">=", 10))
		}
	})

	It("podman search with filter is-official", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-official", "--format", "{{.Official}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal("[OK]"))
		}
	})

	It("podman search with filter is-automated", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-automated=false", "--format", "{{.Automated}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal(""))
		}
	})

	It("podman search attempts HTTP if tls-verify flag is set false", func() {
		fakereg := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry", "registry:2"})
		fakereg.WaitWithDefaultTimeout()
		Expect(fakereg.ExitCode()).To(Equal(0))

		search := podmanTest.Podman([]string{"search", "--registry", "localhost:5000", "fake/image:andtag", "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		// if this test succeeded, there will be no output (there is no entry named fake/image:andtag in an empty registry)
		// and the exit code will be 0
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())

	})

	It("podman search in local registry", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", "docker.io/library/registry:2", "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
		search := podmanTest.Podman([]string{"search", "--registry", "localhost:5000", "my-alpine", "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).ShouldNot(BeEmpty())
	})

	It("podman search from local registry with authorization", func() {
		authPath := filepath.Join(podmanTest.TempDir, "auth")
		os.Mkdir(authPath, os.ModePerm)
		os.MkdirAll("/etc/containers/certs.d/localhost:5000", os.ModePerm)
		debug := podmanTest.SystemExec("ls", []string{"-l", podmanTest.TempDir})
		debug.WaitWithDefaultTimeout()

		cwd, _ := os.Getwd()
		certPath := filepath.Join(cwd, "../", "certs")

		if IsCommandAvailable("getenforce") {
			ge := podmanTest.SystemExec("getenforce", []string{})
			ge.WaitWithDefaultTimeout()
			if ge.OutputToString() == "Enforcing" {
				se := podmanTest.SystemExec("setenforce", []string{"0"})
				se.WaitWithDefaultTimeout()

				defer podmanTest.SystemExec("setenforce", []string{"1"})
			}
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

		push := podmanTest.Podman([]string{"push", "--creds=podmantest:test", "--tls-verify=false", ALPINE, "localhost:5000/tlstest"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		search := podmanTest.Podman([]string{"search", "--registry", "localhost:5000", "tlstest", "--creds=podmantest:test", "--tls-verify=false"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
	})
})
