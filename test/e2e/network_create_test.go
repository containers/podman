package integration

import (
	"encoding/json"
	"net"
	"os"

	"github.com/containers/podman/v3/libpod/network/types"
	. "github.com/containers/podman/v3/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

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

	It("podman network create with name and subnet", func() {
		netName := "subnet-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// JSON the network configuration into something usable
		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Name).To(Equal(netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.11.12.1"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()
		Expect(try).To(Exit(0))

		_, subnet, err := net.ParseCIDR("10.11.12.0/24")
		Expect(err).To(BeNil())
		// Note this is an IPv4 test only!
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
	})

	It("podman network create with name and IPv6 subnet", func() {
		netName := "ipv6-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:1:2:3:4::/64", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// JSON the network configuration into something usable
		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Name).To(Equal(netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("fd00:1:2:3::1"))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("fd00:1:2:3::/64"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		try := podmanTest.Podman([]string{"run", "-it", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  grep global | awk ' /inet6 / {print $2}'"})
		try.WaitWithDefaultTimeout()
		Expect(try).To(Exit(0))

		_, subnet, err := net.ParseCIDR("fd00:1:2:3:4::/64")
		Expect(err).To(BeNil())
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).To(BeNil())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue())
	})

	It("podman network create with name and IPv6 flag (dual-stack)", func() {
		netName := "dual-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:3:2::/64", "--ipv6", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect := podmanTest.Podman([]string{"network", "inspect", netName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// JSON the network configuration into something usable
		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Name).To(Equal(netName))
		Expect(result.Subnets).To(HaveLen(2))
		Expect(result.Subnets[0].Subnet.IP).ToNot(BeNil())
		Expect(result.Subnets[1].Subnet.IP).ToNot(BeNil())

		_, subnet11, err := net.ParseCIDR(result.Subnets[0].Subnet.String())
		Expect(err).To(BeNil())
		_, subnet12, err := net.ParseCIDR(result.Subnets[1].Subnet.String())
		Expect(err).To(BeNil())

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		// create a second network to check the auto assigned ipv4 subnet does not overlap
		// https://github.com/containers/podman/issues/11032
		netName2 := "dual-" + stringid.GenerateNonCryptoID()
		nc = podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:10:3:2::/64", "--ipv6", netName2})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(nc).Should(Exit(0))

		// Inspect the network configuration
		inspect = podmanTest.Podman([]string{"network", "inspect", netName2})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// JSON the network configuration into something usable
		err = json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result = results[0]
		Expect(result.Name).To(Equal(netName2))
		Expect(result.Subnets).To(HaveLen(2))
		Expect(result.Subnets[0].Subnet.IP).ToNot(BeNil())
		Expect(result.Subnets[1].Subnet.IP).ToNot(BeNil())

		_, subnet21, err := net.ParseCIDR(result.Subnets[0].Subnet.String())
		Expect(err).To(BeNil())
		_, subnet22, err := net.ParseCIDR(result.Subnets[1].Subnet.String())
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
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/17000", stringid.GenerateNonCryptoID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with ipv4 subnet and ipv6 flag", func() {
		name := stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--ipv6", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(Exit(0))
		defer podmanTest.removeCNINetwork(name)

		nc = podmanTest.Podman([]string{"network", "inspect", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`::/64`))
		Expect(nc.OutputToString()).To(ContainSubstring(`10.11.12.0/24`))
	})

	It("podman network create with empty subnet and ipv6 flag", func() {
		name := stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--ipv6", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(Exit(0))
		defer podmanTest.removeCNINetwork(name)

		nc = podmanTest.Podman([]string{"network", "inspect", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`::/64`))
		Expect(nc.OutputToString()).To(ContainSubstring(`.0/24`))
	})

	It("podman network create with invalid IP", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.0/17000", stringid.GenerateNonCryptoID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with invalid gateway for subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--gateway", "192.168.1.1", stringid.GenerateNonCryptoID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError())
	})

	It("podman network create two networks with same name should fail", func() {
		netName := "same-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(nc).Should(Exit(0))

		ncFail := podmanTest.Podman([]string{"network", "create", netName})
		ncFail.WaitWithDefaultTimeout()
		Expect(ncFail).To(ExitWithError())
	})

	It("podman network create two networks with same subnet should fail", func() {
		netName1 := "sub1-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(nc).Should(Exit(0))

		netName2 := "sub2-" + stringid.GenerateNonCryptoID()
		ncFail := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName2})
		ncFail.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(ncFail).To(ExitWithError())
	})

	It("podman network create two IPv6 networks with same subnet should fail", func() {
		netName1 := "subipv61-" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:4:4:4::/64", "--ipv6", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(nc).Should(Exit(0))

		netName2 := "subipv62-" + stringid.GenerateNonCryptoID()
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
		net := "mtu-test" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "mtu=9000", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`"mtu": "9000"`))
	})

	It("podman network create with vlan option", func() {
		net := "vlan-test" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "vlan=9", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).To(ContainSubstring(`"vlan": "9"`))
	})

	It("podman network create with invalid option", func() {
		net := "invalid-test" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "foo=bar", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).To(ExitWithError())
	})

	It("podman network create with internal should not have dnsname", func() {
		net := "internal-test" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--internal", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))
		// Not performing this check on remote tests because it is a logrus error which does
		// not come back via stderr on the remote client.
		if !IsRemote() {
			Expect(nc.ErrorToString()).To(ContainSubstring("dnsname and internal networks are incompatible"))
		}
		nc = podmanTest.Podman([]string{"network", "inspect", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
		Expect(nc.OutputToString()).ToNot(ContainSubstring("dnsname"))
	})

	It("podman network create with invalid name", func() {
		for _, name := range []string{"none", "host", "bridge", "private", "slirp4netns", "container", "ns"} {
			nc := podmanTest.Podman([]string{"network", "create", name})
			nc.WaitWithDefaultTimeout()
			Expect(nc).To(Exit(125))
			Expect(nc.ErrorToString()).To(ContainSubstring("cannot create network with name %q because it conflicts with a valid network mode", name))
		}
	})

})
