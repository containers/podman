package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run with --mac-address flag", func() {
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
		// Clean up the CNI networks used by the tests
		os.RemoveAll("/var/lib/cni/networks/podman")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Podman run --mac-address", func() {
		result := podmanTest.Podman([]string{"run", "--mac-address", "92:d0:c6:0a:29:34", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		if isRootless() {
			Expect(result).Should(Exit(125))
		} else {
			Expect(result).Should(Exit(0))
			Expect(result.OutputToString()).To(ContainSubstring("92:d0:c6:0a:29:34"))
		}
	})

	It("Podman run --mac-address with custom network", func() {
		net := "n1" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(Exit(0))

		result := podmanTest.Podman([]string{"run", "--network", net, "--mac-address", "92:d0:c6:00:29:34", ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("92:d0:c6:00:29:34"))
	})
})
