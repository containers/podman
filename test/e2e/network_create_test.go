package integration

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"strings"

	cniversion "github.com/containernetworking/cni/pkg/version"
	"github.com/containers/podman/v3/libpod/network"
	. "github.com/containers/podman/v3/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
		Expect(nc).Should(Exit(0))

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
		Expect(bridgePlugin.IPAM.Ranges[0][0].Gateway).ToNot(BeEmpty())
		Expect(portMapPlugin.Capabilities["portMappings"]).To(BeTrue())

	})

	It("podman network create with name", func() {
		var (
			results []network.NcList
		)

		netName := "inspectnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()

		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal(netName))

	})

	It("podman network create with name and subnet", func() {
		var (
			results []network.NcList
		)
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()

		// JSON the network configuration into something usable
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal(netName))

		// JSON the bridge info
		bridgePlugin, err := genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())
		// check that gateway is added to config
		Expect(bridgePlugin.IPAM.Ranges[0][0].Gateway).To(Equal("10.11.12.1"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(bridgePlugin.BrName)

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()

		_, subnet, err := net.ParseCIDR("10.11.12.0/24")
		Expect(err).To(BeNil())
		// Note this is an IPv4 test only!
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
	})

	It("podman network create with name and IPv6 subnet", func() {
		SkipIfRootless("FIXME It needs the ip6tables modules loaded")
		var (
			results []network.NcList
		)
		netName := "ipv6-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:1:2:3:4::/64", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()

		// JSON the network configuration into something usable
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal(netName))

		// JSON the bridge info
		bridgePlugin, err := genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())
		Expect(bridgePlugin.IPAM.Routes[0].Dest).To(Equal("::/0"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(bridgePlugin.BrName)

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  grep global | awk ' /inet6 / {print $2}'"})
		try.WaitWithDefaultTimeout()

		_, subnet, err := net.ParseCIDR("fd00:1:2:3:4::/64")
		Expect(err).To(BeNil())
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
	})

	It("podman network create with name and IPv6 flag (dual-stack)", func() {
		SkipIfRootless("FIXME It needs the ip6tables modules loaded")
		var (
			results []network.NcList
		)
		netName := "dual-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:3:2:1::/64", "--ipv6", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()

		// JSON the network configuration into something usable
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result := results[0]
		Expect(result["name"]).To(Equal(netName))

		// JSON the bridge info
		bridgePlugin, err := genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())
		Expect(bridgePlugin.IPAM.Routes[0].Dest).To(Equal("::/0"))
		Expect(bridgePlugin.IPAM.Routes[1].Dest).To(Equal("0.0.0.0/0"))

		Expect(bridgePlugin.IPAM.Ranges).To(HaveLen(2))
		Expect(bridgePlugin.IPAM.Ranges[0]).To(HaveLen(1))
		Expect(bridgePlugin.IPAM.Ranges[0][0].Subnet).ToNot(BeEmpty())
		Expect(bridgePlugin.IPAM.Ranges[1]).To(HaveLen(1))
		Expect(bridgePlugin.IPAM.Ranges[1][0].Subnet).ToNot(BeEmpty())

		_, subnet11, err := net.ParseCIDR(bridgePlugin.IPAM.Ranges[0][0].Subnet)
		Expect(err).To(BeNil())
		_, subnet12, err := net.ParseCIDR(bridgePlugin.IPAM.Ranges[1][0].Subnet)
		Expect(err).To(BeNil())

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(bridgePlugin.BrName)

		// create a second network to check the auto assigned ipv4 subnet does not overlap
		// https://github.com/containers/podman/issues/11032
		netName2 := "dual-" + stringid.GenerateRandomID()
		nc = podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:6:3:2:1::/64", "--ipv6", netName2})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect = podmanTest.Podman([]string{"network", "inspect", netName2})
		inspect.WaitWithDefaultTimeout()

		// JSON the network configuration into something usable
		err = json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		result = results[0]
		Expect(result["name"]).To(Equal(netName2))

		// JSON the bridge info
		bridgePlugin, err = genericPluginsToBridge(result["plugins"], "bridge")
		Expect(err).To(BeNil())
		Expect(bridgePlugin.IPAM.Routes[0].Dest).To(Equal("::/0"))
		Expect(bridgePlugin.IPAM.Routes[1].Dest).To(Equal("0.0.0.0/0"))
		Expect(bridgePlugin.IPAM.Ranges).To(HaveLen(2))
		Expect(bridgePlugin.IPAM.Ranges[0]).To(HaveLen(1))
		Expect(bridgePlugin.IPAM.Ranges[0][0].Subnet).ToNot(BeEmpty())
		Expect(bridgePlugin.IPAM.Ranges[1]).To(HaveLen(1))
		Expect(bridgePlugin.IPAM.Ranges[1][0].Subnet).ToNot(BeEmpty())

		_, subnet21, err := net.ParseCIDR(bridgePlugin.IPAM.Ranges[0][0].Subnet)
		Expect(err).To(BeNil())
		_, subnet22, err := net.ParseCIDR(bridgePlugin.IPAM.Ranges[1][0].Subnet)
		Expect(err).To(BeNil())

		// check that the subnets do not overlap
		Expect(subnet11.Contains(subnet21.IP)).To(BeFalse())
		Expect(subnet12.Contains(subnet22.IP)).To(BeFalse())

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  grep global | awk ' /inet6 / {print $2}'"})
		try.WaitWithDefaultTimeout()

		_, subnet, err := net.ParseCIDR("fd00:4:3:2:1::/64")
		Expect(err).To(BeNil())
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
		// verify the container has an IPv4 address too (the IPv4 subnet is autogenerated)
		try = podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()
		containerIP, _, err = net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		Expect(containerIP.To4()).To(Not(BeNil()))
	})

	It("podman network create with invalid subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/17000", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with ipv4 subnet and ipv6 flag", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--ipv6", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with empty subnet and ipv6 flag", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--ipv6", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with invalid IP", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.0/17000", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with invalid gateway for subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--gateway", "192.168.1.1", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create two networks with same name should fail", func() {
		netName := "same-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		ncFail := podmanTest.Podman([]string{"network", "create", netName})
		ncFail.WaitWithDefaultTimeout()
		Expect(ncFail).To(ExitWithError())
	})

	It("podman network create two networks with same subnet should fail", func() {
		netName1 := "sub1-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(nc).Should(Exit(0))

		netName2 := "sub2-" + stringid.GenerateRandomID()
		ncFail := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName2})
		ncFail.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(ncFail).To(ExitWithError())
	})

	It("podman network create two IPv6 networks with same subnet should fail", func() {
		SkipIfRootless("FIXME It needs the ip6tables modules loaded")
		netName1 := "subipv61-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:4:4:4::/64", "--ipv6", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(nc).Should(Exit(0))

		netName2 := "subipv62-" + stringid.GenerateRandomID()
		ncFail := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:4:4:4::/64", "--ipv6", netName2})
		ncFail.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(ncFail).To(ExitWithError())
	})

	It("podman network create with invalid network name", func() {
		nc := podmanTest.Podman([]string{"network", "create", "foo "})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with mtu option", func() {
		net := "mtu-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "mtu=9000", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`"mtu": 9000,`))
	})

	It("podman network create with vlan option", func() {
		net := "vlan-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "vlan=9", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`"vlan": 9`))
	})

	It("podman network create with invalid option", func() {
		net := "invalid-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "foo=bar", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with internal should not have dnsname", func() {
		net := "internal-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--internal", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))
		// Not performing this check on remote tests because it is a logrus error which does
		// not come back via stderr on the remote client.
		if !IsRemote() {
			Expect(nc.ErrorToString()).To(ContainSubstring("dnsname and --internal networks are incompatible"))
		}
		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).ToNot(ContainSubstring("dnsname"))
	})
})
