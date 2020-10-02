package integration

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/podman/v2/pkg/rootless"
	. "github.com/containers/podman/v2/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func writeConf(conf []byte, confPath string) {
	if err := ioutil.WriteFile(confPath, conf, 777); err != nil {
		fmt.Println(err)
	}
}
func removeConf(confPath string) {
	if err := os.Remove(confPath); err != nil {
		fmt.Println(err)
	}
}

// generateNetworkConfig generates a cni config with a random name
// it returns the network name and the filepath
func generateNetworkConfig(p *PodmanTestIntegration) (string, string) {
	// generate a random name to preven conflicts with other tests
	name := "net" + stringid.GenerateNonCryptoID()
	path := filepath.Join(p.CNIConfigDir, fmt.Sprintf("%s.conflist", name))
	conf := fmt.Sprintf(`{
		"cniVersion": "0.3.0",
		"name": "%s",
		"plugins": [
		  {
			"type": "bridge",
			"bridge": "cni1",
			"isGateway": true,
			"ipMasq": true,
			"ipam": {
				"type": "host-local",
				"subnet": "%s",
				"routes": [
					{ "dst": "0.0.0.0/0" }
				]
			}
		  },
		  {
			"type": "portmap",
			"capabilities": {
			  "portMappings": true
			}
		  }
		]
	}`, name, p.GetSafeIPv4Subnet())
	writeConf([]byte(conf), path)

	return name, path
}

var _ = Describe("Podman network", func() {
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
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman network list", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list -q", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list --filter success", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=bridge"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list --filter failure", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeFalse())
	})

	It("podman network rm no args", func() {
		session := podmanTest.Podman([]string{"network", "rm"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network rm", func() {
		SkipIfRootless("FIXME: This one is definitely broken in rootless mode")
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())

		rm := podmanTest.Podman([]string{"network", "rm", name})
		rm.WaitWithDefaultTimeout()
		Expect(rm.ExitCode()).To(BeZero())

		results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		Expect(results.LineInOutputContains(name)).To(BeFalse())
	})

	It("podman network inspect no args", func() {
		session := podmanTest.Podman([]string{"network", "inspect"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		expectedNetworks := []string{name}
		if !rootless.IsRootless() {
			// rootful image contains "podman/cni/87-podman-bridge.conflist" for "podman" network
			expectedNetworks = append(expectedNetworks, "podman")
		}
		session := podmanTest.Podman(append([]string{"network", "inspect"}, expectedNetworks...))
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "inspect", name, "--format", "{{.cniVersion}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains("0.3.0")).To(BeTrue())
	})

	It("podman inspect container single CNI network", func() {
		netName := "testNetSingleCNI"
		subnet := podmanTest.GetSafeIPv4Subnet()
		network := podmanTest.Podman([]string{"network", "create", "--subnet", subnet, netName})
		network.WaitWithDefaultTimeout()
		Expect(network.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", netName, "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(BeZero())
		conData := inspect.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(1))
		net1, ok := conData[0].NetworkSettings.Networks[netName]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName))
		Expect(net1.IPPrefixLen).To(Equal(24))
		_, ipNet, err := net.ParseCIDR(subnet)
		Expect(err).To(BeNil())
		Expect(ipNet.Contains(net.ParseIP(net1.IPAddress))).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})

	It("podman inspect container two CNI networks", func() {
		netName1 := "testNetTwoCNI1"
		subnet1 := podmanTest.GetSafeIPv4Subnet()
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, netName1})
		network1.WaitWithDefaultTimeout()
		Expect(network1.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName1)

		netName2 := "testNetTwoCNI2"
		subnet2 := podmanTest.GetSafeIPv4Subnet()
		network2 := podmanTest.Podman([]string{"network", "create", "--subnet", subnet2, netName2})
		network2.WaitWithDefaultTimeout()
		Expect(network2.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName2)

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(BeZero())
		conData := inspect.InspectContainerToJSON()
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		Expect(net1.IPPrefixLen).To(Equal(24))
		_, ipNet, err := net.ParseCIDR(subnet1)
		Expect(err).To(BeNil())
		Expect(ipNet.Contains(net.ParseIP(net1.IPAddress))).To(BeTrue())
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))
		Expect(net2.IPPrefixLen).To(Equal(24))
		_, ipNet, err = net.ParseCIDR(subnet2)
		Expect(err).To(BeNil())
		Expect(ipNet.Contains(net.ParseIP(net2.IPAddress))).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})

	It("podman network remove bogus", func() {
		session := podmanTest.Podman([]string{"network", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))
	})

	It("podman network remove --force with pod", func() {
		netName := "testnet"
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		session = podmanTest.Podman([]string{"pod", "create", "--network", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", podID, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())

		session = podmanTest.Podman([]string{"network", "rm", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(2))

		session = podmanTest.Podman([]string{"network", "rm", "--force", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())

		// check if pod is deleted
		session = podmanTest.Podman([]string{"pod", "exists", podID})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(1))

		// check if net is deleted
		session = podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		Expect(session.OutputToString()).To(Not(ContainSubstring(netName)))
	})

	It("podman network remove with two networks", func() {
		netName1 := "net1"
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName1)

		netName2 := "net2"
		session = podmanTest.Podman([]string{"network", "create", netName2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName2)

		session = podmanTest.Podman([]string{"network", "rm", netName1, netName2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		lines := session.OutputToStringArray()
		Expect(lines[0]).To(Equal(netName1))
		Expect(lines[1]).To(Equal(netName2))
	})
})
