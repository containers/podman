package integration

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/podman/v4/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman system service", func() {
	var podmanTest *PodmanTestIntegration

	// The timeout used to for the service to respond. As shown in #12167,
	// this may take some time on machines under high load.
	var timeout = 20

	BeforeEach(func() {
		tempdir, err := CreateTempDirInTempDir()
		Expect(err).ShouldNot(HaveOccurred())

		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		processTestResult(CurrentGinkgoTestDescription())
	})

	Describe("verify timeout", func() {
		It("of 2 seconds", func() {
			SkipIfRemote("service subcommand not supported remotely")

			address := url.URL{
				Scheme: "tcp",
				Host:   net.JoinHostPort("localhost", randomPort()),
			}
			session := podmanTest.Podman([]string{
				"system", "service", "--time=2", address.String(),
			})
			defer session.Kill()

			WaitForService(address)
			Eventually(session, timeout).Should(Exit(0))
		})
	})

	Describe("verify pprof endpoints", func() {
		// Depends on pkg/api/server/server.go:255
		const magicComment = "pprof service listening on"

		It("are available", func() {
			SkipIfRemote("service subcommand not supported remotely")

			address := url.URL{
				Scheme: "tcp",
				Host:   net.JoinHostPort("localhost", randomPort()),
			}

			pprofPort := randomPort()
			session := podmanTest.Podman([]string{
				"system", "service", "--log-level=debug", "--time=0",
				"--pprof-address=localhost:" + pprofPort, address.String(),
			})
			defer session.Kill()

			WaitForService(address)

			// Combined with test below we have positive/negative test for pprof
			Expect(session.Err.Contents()).Should(ContainSubstring(magicComment))

			heap := url.URL{
				Scheme:   "http",
				Host:     net.JoinHostPort("localhost", pprofPort),
				Path:     "/debug/pprof/heap",
				RawQuery: "seconds=2",
			}
			resp, err := http.Get(heap.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp).To(HaveHTTPStatus(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(body).ShouldNot(BeEmpty())

			session.Interrupt().Wait(time.Duration(timeout) * time.Second)
			Eventually(session, timeout).Should(Exit(1))
		})

		It("are not available", func() {
			SkipIfRemote("service subcommand not supported remotely")

			address := url.URL{
				Scheme: "tcp",
				Host:   net.JoinHostPort("localhost", randomPort()),
			}

			session := podmanTest.Podman([]string{
				"system", "service", "--log-level=debug", "--time=0", address.String(),
			})
			defer session.Kill()

			WaitForService(address)

			// Combined with test above we have positive/negative test for pprof
			Expect(session.Err.Contents()).ShouldNot(ContainSubstring(magicComment))

			session.Interrupt().Wait(time.Duration(timeout) * time.Second)
			Eventually(session, timeout).Should(Exit(1))
		})
	})
})

// WaitForService blocks, waiting for some service listening on given host:port
func WaitForService(address url.URL) {
	// Wait for podman to be ready
	var conn net.Conn
	var err error
	for i := 1; i <= 5; i++ {
		conn, err = net.Dial("tcp", address.Host)
		if err != nil {
			// Podman not available yet...
			time.Sleep(time.Duration(i) * time.Second)
		}
	}
	Expect(err).ShouldNot(HaveOccurred())
	conn.Close()
}

// randomPort leans on the go net library to find an available port...
func randomPort() string {
	port, err := utils.GetRandomPort()
	Expect(err).ShouldNot(HaveOccurred())
	return strconv.Itoa(port)
}
