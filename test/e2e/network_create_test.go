// +build !remoteclient

package integration

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"strings"

	cniversion "github.com/containernetworking/cni/pkg/version"
	"github.com/containers/libpod/pkg/network"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var ErrPluginNotFound = errors.New("plugin not found")

func findPluginByName(plugins interface{}, pluginType string) (interface{}, error) {
	for _, p := range plugins.([]interface{}) {
		r := p.(map[string]interface{})
		if pluginType == r["type"] {
			return p, nil
		}
	}
	return nil, errors.Wrap(ErrPluginNotFound, pluginType)
}

func genericPluginsToBridge(plugins interface{}, pluginType string) (network.HostLocalBridge, error) {
	var bridge network.HostLocalBridge
	generic, err := findPluginByName(plugins, pluginType)
	if err != nil {
		return bridge, err
	}
	b, err := json.Marshal(generic)
	if err != nil {
		return bridge, err
	}
	err = json.Unmarshal(b, &bridge)
	return bridge, err
}

func genericPluginsToPortMap(plugins interface{}, pluginType string) (network.PortMapConfig, error) {
	var portMap network.PortMapConfig
	generic, err := findPluginByName(plugins, "portmap")
	if err != nil {
		return portMap, err
	}
	b, err := json.Marshal(generic)
	if err != nil {
		return portMap, err
	}
	err = json.Unmarshal(b, &portMap)
	return portMap, err
}

func (p *PodmanTestIntegration) removeCNINetwork(name string) {
	session := p.Podman([]string{"network", "rm", name})
	session.WaitWithDefaultTimeout()
	Expect(session.ExitCode()).To(BeZero())
}

func removeNetworkDevice(name string) {
	session := SystemExec("ip", []string{"link", "delete", name})
	session.WaitWithDefaultTimeout()
}

var _ = Describe("Podman network create", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman network create with no input", func() {
		var result network.NcList

		nc := podmanTest.Podman([]string{"network", "create"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).To(BeZero())

		fileContent, err := ioutil.ReadFile(nc.OutputToString())
		Expect(err).To(BeNil())
		err = json.Unmarshal(fileContent, &result)
		Expect(err).To(BeNil())
		defer podmanTest.removeCNINetwork(result["name"].(string))
		Expect(result["cniVersion"]).To(Equal(cniversion.Current()))
		Expect(strings.HasPrefix(result["name"].(string), "cni-podman")).To(BeTrue())

		bridgePlugin, err := genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())
		portMapPlugin, err := genericPluginsToPortMap(result["plugins"], "portmap")
		Expect(err).To(BeNil())

		Expect(bridgePlugin.IPAM.Routes[0].Dest).To(Equal("0.0.0.0/0"))
		Expect(bridgePlugin.IsGW).To(BeTrue())
		Expect(bridgePlugin.IPMasq).To(BeTrue())
		Expect(portMapPlugin.Capabilities["portMappings"]).To(BeTrue())

	})

	It("podman network create with name", func() {
		var (
			results []network.NcList
		)

		nc := podmanTest.Podman([]string{"network", "create", "newname"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork("newname")

		inspect := podmanTest.Podman([]string{"network", "inspect", "newname"})
		inspect.WaitWithDefaultTimeout()

		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal("newname"))

	})

	It("podman network create with name and subnet", func() {
		var (
			results []network.NcList
		)
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "newnetwork"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).To(BeZero())

		defer podmanTest.removeCNINetwork("newnetwork")

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", "newnetwork"})
		inspect.WaitWithDefaultTimeout()

		// JSON the network configuration into something usable
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal("newnetwork"))

		// JSON the bridge info
		bridgePlugin, err := genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(bridgePlugin.BrName)

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", "newnetwork", ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()

		_, subnet, err := net.ParseCIDR("10.11.12.0/24")
		Expect(err).To(BeNil())
		// Note this is an IPv4 test only!
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
	})

	It("podman network create with invalid subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/17000", "fail"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).ToNot(BeZero())
	})

	It("podman network create with invalid IP", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.0/17000", "fail"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).ToNot(BeZero())
	})

	It("podman network create with invalid gateway for subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--gateway", "192.168.1.1", "fail"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).ToNot(BeZero())
	})

	It("podman network create two networks with same name should fail", func() {
		nc := podmanTest.Podman([]string{"network", "create", "samename"})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork("samename")

		ncFail := podmanTest.Podman([]string{"network", "create", "samename"})
		ncFail.WaitWithDefaultTimeout()
		Expect(ncFail.ExitCode()).ToNot(BeZero())
	})

	It("podman network create with invalid network name", func() {
		nc := podmanTest.Podman([]string{"network", "create", "foo "})
		nc.WaitWithDefaultTimeout()
		Expect(nc.ExitCode()).ToNot(BeZero())
	})

})
