package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman login and logout", func() {
	var (
		tempdir                  string
		err                      error
		podmanTest               *PodmanTestIntegration
		authPath                 string
		certPath                 string
		certDirPath              string
		server                   string
		testImg                  string
		registriesConfWithSearch []byte
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)

		authPath = filepath.Join(podmanTest.TempDir, "auth")
		err := os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsCommandAvailable("getenforce") {
			ge := SystemExec("getenforce", []string{})
			ge.WaitWithDefaultTimeout()
			if ge.OutputToString() == "Enforcing" {
				se := SystemExec("setenforce", []string{"0"})
				se.WaitWithDefaultTimeout()
				if se.ExitCode() != 0 {
					Skip("Cannot disable selinux, this may cause problem for reading cert files inside container.")
				}
				defer SystemExec("setenforce", []string{"1"})
			}
		}

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
		port := GetPort()
		server = strings.Join([]string{"localhost", strconv.Itoa(port)}, ":")

		registriesConfWithSearch = []byte(fmt.Sprintf("[registries.search]\nregistries = ['%s']", server))

		testImg = strings.Join([]string{server, "test-alpine"}, "/")

		certDirPath = filepath.Join(os.Getenv("HOME"), ".config/containers/certs.d", server)
		err = os.MkdirAll(certDirPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		cwd, _ := os.Getwd()
		certPath = filepath.Join(cwd, "../", "certs")

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), filepath.Join(certDirPath, "ca.crt")})
		setup.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"run", "-d", "-p", strings.Join([]string{strconv.Itoa(port), strconv.Itoa(port)}, ":"),
			"-e", strings.Join([]string{"REGISTRY_HTTP_ADDR=0.0.0.0", strconv.Itoa(port)}, ":"), "--name", "registry", "-v",
			strings.Join([]string{authPath, "/auth:Z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs:Z"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		os.RemoveAll(authPath)
		os.RemoveAll(certDirPath)
	})

	readAuthInfo := func(filePath string) map[string]interface{} {
		authBytes, err := os.ReadFile(filePath)
		Expect(err).ToNot(HaveOccurred())

		var authInfo map[string]interface{}
		err = json.Unmarshal(authBytes, &authInfo)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(authInfo)

		const authsKey = "auths"
		Expect(authInfo).To(HaveKey(authsKey))

		auths, ok := authInfo[authsKey].(map[string]interface{})
		Expect(ok).To(BeTrue())

		return auths
	}

	It("podman login and logout", func() {
		session := podmanTest.Podman([]string{"login", "-u", "podmantest", "-p", "test", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman login and logout without registry parameter", func() {
		registriesConf, err := os.CreateTemp("", "TestLoginWithoutParameter")
		Expect(err).ToNot(HaveOccurred())
		defer registriesConf.Close()
		defer os.Remove(registriesConf.Name())

		err = os.WriteFile(registriesConf.Name(), registriesConfWithSearch, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		// Environment is per-process, so this looks very unsafe; actually it seems fine because tests are not
		// run in parallel unless they opt in by calling t.Parallel().  So don’t do that.
		oldRCP, hasRCP := os.LookupEnv("CONTAINERS_REGISTRIES_CONF")
		defer func() {
			if hasRCP {
				os.Setenv("CONTAINERS_REGISTRIES_CONF", oldRCP)
			} else {
				os.Unsetenv("CONTAINERS_REGISTRIES_CONF")
			}
		}()
		os.Setenv("CONTAINERS_REGISTRIES_CONF", registriesConf.Name())

		session := podmanTest.Podman([]string{"login", "-u", "podmantest", "-p", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman login and logout with flag --authfile", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		readAuthInfo(authFile)

		// push should fail with nonexistent authfile
		session = podmanTest.Podman([]string{"push", "--authfile", "/tmp/nonexistent", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"push", "--authfile", authFile, ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--authfile", authFile, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// logout should fail with nonexistent authfile
		session = podmanTest.Podman([]string{"logout", "--authfile", "/tmp/nonexistent", server})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"logout", "--authfile", authFile, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman login and logout with --tls-verify", func() {
		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--tls-verify=false", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman login and logout with --cert-dir", func() {
		certDir := filepath.Join(podmanTest.TempDir, "certs")
		err := os.MkdirAll(certDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), filepath.Join(certDir, "ca.crt")})
		setup.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "--cert-dir", certDir, server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", "--cert-dir", certDir, ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
	It("podman login and logout with multi registry", func() {
		certDir := filepath.Join(os.Getenv("HOME"), ".config/containers/certs.d", "localhost:9001")
		err = os.MkdirAll(certDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		cwd, _ := os.Getwd()
		certPath = filepath.Join(cwd, "../", "certs")

		setup := SystemExec("cp", []string{filepath.Join(certPath, "domain.crt"), filepath.Join(certDir, "ca.crt")})
		setup.WaitWithDefaultTimeout()
		defer os.RemoveAll(certDir)

		// N/B: This second registry container shares the same auth and cert dirs
		//      as the registry started from BeforeEach().  Since this one starts
		//      second, re-labeling the volumes should keep SELinux happy.
		session := podmanTest.Podman([]string{"run", "-d", "-p", "9001:9001", "-e", "REGISTRY_HTTP_ADDR=0.0.0.0:9001", "--name", "registry1", "-v",
			strings.Join([]string{authPath, "/auth:z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			"-v", strings.Join([]string{certPath, "/certs:z"}, ":"), "-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key", REGISTRY_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry1", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout", server})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"login", "--username", "podmantest", "--password", "test", "localhost:9001"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"logout", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"push", ALPINE, testImg})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"push", ALPINE, "localhost:9001/test-alpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman login and logout with repository", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testRepository := server + "/podmantest"
		session := podmanTest.Podman([]string{
			"login",
			"-u", "podmantest",
			"-p", "test",
			"--authfile", authFile,
			testRepository,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testRepository))

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepository,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		authInfo = readAuthInfo(authFile)
		Expect(authInfo).NotTo(HaveKey(testRepository))
	})

	It("podman login and logout with repository and specified image", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testTarget := server + "/podmantest/test-alpine"
		session := podmanTest.Podman([]string{
			"login",
			"-u", "podmantest",
			"-p", "test",
			"--authfile", authFile,
			testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testTarget))

		session = podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

	})

	It("podman login and logout with repository with fallback", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testRepos := []string{
			server + "/podmantest",
			server,
		}
		for _, testRepo := range testRepos {
			session := podmanTest.Podman([]string{
				"login",
				"-u", "podmantest",
				"-p", "test",
				"--authfile", authFile,
				testRepo,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}

		authInfo := readAuthInfo(authFile)
		Expect(authInfo).To(HaveKey(testRepos[0]))
		Expect(authInfo).To(HaveKey(testRepos[1]))

		session := podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, testRepos[0] + "/test-image-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepos[0],
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, testRepos[0] + "/test-image-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{
			"logout",
			"--authfile", authFile,
			testRepos[1],
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		authInfo = readAuthInfo(authFile)
		Expect(authInfo).NotTo(HaveKey(testRepos[0]))
		Expect(authInfo).NotTo(HaveKey(testRepos[1]))
	})

	It("podman login with http{s} prefix", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		for _, invalidArg := range []string{
			"https://" + server + "/podmantest",
			"http://" + server + "/podmantest/image:latest",
		} {
			session := podmanTest.Podman([]string{
				"login",
				"-u", "podmantest",
				"-p", "test",
				"--authfile", authFile,
				invalidArg,
			})
			session.WaitWithDefaultTimeout()
			Expect(session).To(Exit(0))
		}
	})

	It("podman login and logout with repository push with invalid auth.json credentials", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		// only `server` contains the correct login data
		err := os.WriteFile(authFile, []byte(fmt.Sprintf(`{"auths": {
			"%s/podmantest": { "auth": "cG9kbWFudGVzdDp3cm9uZw==" },
			"%s": { "auth": "cG9kbWFudGVzdDp0ZXN0" }
		}}`, server, server)), 0644)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, server + "/podmantest/test-image",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, server + "/test-image",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))
	})

	It("podman login and logout with repository pull with wrong auth.json credentials", func() {
		authFile := filepath.Join(podmanTest.TempDir, "auth.json")

		testTarget := server + "/podmantest/test-alpine"
		session := podmanTest.Podman([]string{
			"login",
			"-u", "podmantest",
			"-p", "test",
			"--authfile", authFile,
			testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{
			"push",
			"--authfile", authFile,
			ALPINE, testTarget,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// only `server + /podmantest` and `server` have the correct login data
		err := os.WriteFile(authFile, []byte(fmt.Sprintf(`{"auths": {
			"%s/podmantest/test-alpine": { "auth": "cG9kbWFudGVzdDp3cm9uZw==" },
			"%s/podmantest": { "auth": "cG9kbWFudGVzdDp0ZXN0" },
			"%s": { "auth": "cG9kbWFudGVzdDp0ZXN0" }
		}}`, server, server, server)), 0644)
		Expect(err).ToNot(HaveOccurred())

		session = podmanTest.Podman([]string{
			"pull",
			"--authfile", authFile,
			server + "/podmantest/test-alpine",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})
})
