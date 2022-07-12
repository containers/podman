package integration

import (
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run with --ip flag", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless("rootless does not support --ip without network")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		// Clean up the CNI networks used by the tests
		os.RemoveAll("/var/lib/cni/networks/podman")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Podman run --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run --ip with v6 address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "2001:db8:bad:beef::1", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run --ip with non-allocatable IP", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run with specified static IP has correct IP", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with specified static IPv6 has correct IP", func() {
		netName := "ipv6-" + stringid.GenerateNonCryptoID()
		ipv6 := "fd46:db93:aa76:ac37::10"
		net := podmanTest.Podman([]string{"network", "create", "--subnet", "fd46:db93:aa76:ac37::/64", netName})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(net).To(Exit(0))

		result := podmanTest.Podman([]string{"run", "-ti", "--network", netName, "--ip6", ipv6, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ipv6 + "/64"))
	})

	It("Podman run with --network bridge:ip=", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-ti", "--network", "bridge:ip=" + ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with --network net:ip=,mac=,interface_name=", func() {
		ip := GetRandomIPAddress()
		mac := "44:33:22:11:00:99"
		intName := "myeth"
		result := podmanTest.Podman([]string{"run", "-ti", "--network", "bridge:ip=" + ip + ",mac=" + mac + ",interface_name=" + intName, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
		Expect(result.OutputToString()).To(ContainSubstring(mac))
		Expect(result.OutputToString()).To(ContainSubstring(intName))
	})

	It("Podman run two containers with the same IP", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-d", "--name", "nginx", "--ip", ip, NGINX_IMAGE})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		for retries := 20; retries > 0; retries-- {
			response, err := http.Get(fmt.Sprintf("http://%s", ip))
			if err == nil && response.StatusCode == http.StatusOK {
				break
			}
			if retries == 1 {
				logps := podmanTest.Podman([]string{"ps", "-a"})
				logps.WaitWithDefaultTimeout()
				logps = podmanTest.Podman([]string{"logs", "nginx"})
				logps.WaitWithDefaultTimeout()
				Fail("Timed out waiting for nginx container, see ps & log above.")
			}

			if err != nil {
				fmt.Printf("nginx not ready yet; error=%v; %d retries left...\n", err, retries)
			} else {
				fmt.Printf("nginx not ready yet; response=%v; %d retries left...\n", response.StatusCode, retries)
			}
			time.Sleep(1 * time.Second)
		}
		result = podmanTest.Podman([]string{"run", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
		Expect(result.ErrorToString()).To(ContainSubstring(" address %s ", ip))
	})
})
