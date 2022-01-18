package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/pkg/rootless"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

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

	It("podman --cni-config-dir backwards compat", func() {
		SkipIfRemote("--cni-config-dir only works locally")
		netDir, err := CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(netDir)
		session := podmanTest.Podman([]string{"--cni-config-dir", netDir, "network", "ls", "--noheading"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// default network always exists
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// check that the only file in the directory is the network lockfile
		dir, err := os.Open(netDir)
		Expect(err).ToNot(HaveOccurred())
		names, err := dir.Readdirnames(5)
		Expect(err).ToNot(HaveOccurred())
		Expect(names).To(HaveLen(1))
		Expect(names[0]).To(Or(Equal("netavark.lock"), Equal("cni.lock")))
	})

	It("podman network list", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list -q", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter success", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "driver=bridge"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter driver and name", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "driver=bridge", "--filter", "name=" + name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter two names", func() {
		name1, path1 := generateNetworkConfig(podmanTest)
		defer removeConf(path1)

		name2, path2 := generateNetworkConfig(podmanTest)
		defer removeConf(path2)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "name=" + name1, "--filter", "name=" + name2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name1))
		Expect(session.OutputToString()).To(ContainSubstring(name2))
	})

	It("podman network list --filter labels", func() {
		net1 := "labelnet" + stringid.GenerateNonCryptoID()
		label1 := "testlabel1=abc"
		label2 := "abcdef"
		session := podmanTest.Podman([]string{"network", "create", "--label", label1, net1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net1)
		Expect(session).Should(Exit(0))

		net2 := "labelnet" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", "--label", label1, "--label", label2, net2})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net2)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1, "--filter", "label=" + label2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))
	})

	It("podman network list --filter invalid value", func() {
		net := "net" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "namr=ab"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring(`invalid filter "namr"`))
	})

	It("podman network list --filter failure", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "label=abc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring(name)))
	})

	It("podman network ID test", func() {
		net := "networkIDTest"
		// the network id should be the sha256 hash of the network name
		netID := "6073aefe03cdf8f29be5b23ea9795c431868a3a22066a6290b187691614fee84"
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		// Tests Default Table Output
		session = podmanTest.Podman([]string{"network", "ls", "--filter", "id=" + netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		expectedTable := "NETWORK ID NAME DRIVER"
		Expect(session.OutputToString()).To(ContainSubstring(expectedTable))

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}} {{.ID}}", "--filter", "id=" + netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(net + " " + netID[:12]))

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}} {{.ID}}", "--filter", "id=" + netID[10:50], "--no-trunc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(net + " " + netID))

		session = podmanTest.Podman([]string{"network", "inspect", netID[:40]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(net))

		session = podmanTest.Podman([]string{"network", "inspect", netID[1:]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("network not found"))

		session = podmanTest.Podman([]string{"network", "rm", netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	rmFunc := func(rm string) {
		It(fmt.Sprintf("podman network %s no args", rm), func() {
			session := podmanTest.Podman([]string{"network", rm})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError())

		})

		It(fmt.Sprintf("podman network %s", rm), func() {
			name, path := generateNetworkConfig(podmanTest)
			defer removeConf(path)

			session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			Expect(session.OutputToString()).To(ContainSubstring(name))

			rm := podmanTest.Podman([]string{"network", rm, name})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(Exit(0))

			results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			results.WaitWithDefaultTimeout()
			Expect(results).Should(Exit(0))
			Expect(results.OutputToString()).To(Not(ContainSubstring(name)))
		})
	}

	rmFunc("rm")
	rmFunc("remove")

	It("podman network inspect no args", func() {
		session := podmanTest.Podman([]string{"network", "inspect"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
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
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "inspect", name, "--format", "{{.Driver}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("bridge"))
	})

	It("podman inspect container single CNI network", func() {
		netName := "net-" + stringid.GenerateNonCryptoID()
		network := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.50.0/24", netName})
		network.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(network).Should(Exit(0))

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", netName, "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(1))
		net, ok := conData[0].NetworkSettings.Networks[netName]
		Expect(ok).To(BeTrue())
		Expect(net.NetworkID).To(Equal(netName))
		Expect(net.IPPrefixLen).To(Equal(24))
		Expect(net.IPAddress).To(HavePrefix("10.50.50."))

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman inspect container two CNI networks (container not running)", func() {
		netName1 := "net1-" + stringid.GenerateNonCryptoID()
		network1 := podmanTest.Podman([]string{"network", "create", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(network1).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateNonCryptoID()
		network2 := podmanTest.Podman([]string{"network", "create", netName2})
		network2.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(network2).Should(Exit(0))

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"create", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman inspect container two CNI networks", func() {
		netName1 := "net1-" + stringid.GenerateNonCryptoID()
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.0/25", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(network1).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateNonCryptoID()
		network2 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.128/26", netName2})
		network2.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(network2).Should(Exit(0))

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		Expect(net1.IPPrefixLen).To(Equal(25))
		Expect(net1.IPAddress).To(HavePrefix("10.50.51."))
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))
		Expect(net2.IPPrefixLen).To(Equal(26))
		Expect(net2.IPAddress).To(HavePrefix("10.50.51."))

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman network remove after disconnect when container initially created with the network", func() {
		container := "test"
		network := "foo" + stringid.GenerateNonCryptoID()

		session := podmanTest.Podman([]string{"network", "create", network})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(network)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", container, "--network", network, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "disconnect", network, container})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "rm", network})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman network remove bogus", func() {
		session := podmanTest.Podman([]string{"network", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman network remove --force with pod", func() {
		netName := "net-" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pod", "create", "--network", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", podID, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "rm", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(2))

		session = podmanTest.Podman([]string{"network", "rm", "-t", "0", "--force", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// check if pod is deleted
		session = podmanTest.Podman([]string{"pod", "exists", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))

		// check if net is deleted
		session = podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring(netName)))
	})

	It("podman network remove with two networks", func() {
		netName1 := "net1-" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(session).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", netName2})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName2)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "rm", netName1, netName2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		lines := session.OutputToStringArray()
		Expect(lines[0]).To(Equal(netName1))
		Expect(lines[1]).To(Equal(netName2))
	})

	It("podman network with multiple aliases", func() {
		var worked bool
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName)
		Expect(session).Should(Exit(0))

		interval := time.Duration(250 * time.Millisecond)
		for i := 0; i < 6; i++ {
			n := podmanTest.Podman([]string{"network", "exists", netName})
			n.WaitWithDefaultTimeout()
			worked = n.ExitCode() == 0
			if worked {
				break
			}
			time.Sleep(interval)
			interval *= 2
		}

		top := podmanTest.Podman([]string{"run", "-dt", "--name=web", "--network=" + netName, "--network-alias=web1", "--network-alias=web2", nginx})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(Exit(0))
		interval = time.Duration(250 * time.Millisecond)
		// Wait for the nginx service to be running
		for i := 0; i < 6; i++ {
			// Test curl against the container's name
			c1 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, nginx, "curl", "web"})
			c1.WaitWithDefaultTimeout()
			worked = c1.ExitCode() == 0
			if worked {
				break
			}
			time.Sleep(interval)
			interval *= 2
		}
		Expect(worked).To(BeTrue())

		// Nginx is now running so no need to do a loop
		// Test against the first alias
		c2 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, nginx, "curl", "web1"})
		c2.WaitWithDefaultTimeout()
		Expect(c2).Should(Exit(0))

		// Test against the second alias
		c3 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, nginx, "curl", "web2"})
		c3.WaitWithDefaultTimeout()
		Expect(c3).Should(Exit(0))
	})

	It("podman network create/remove macvlan", func() {
		net := "macvlan" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "--macvlan", "lo", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network create/remove macvlan as driver (-d) no device name", func() {
		net := "macvlan" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// JSON the network configuration into something usable
		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.NetworkInterface).To(Equal(""))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "dhcp"))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network create/remove macvlan as driver (-d) with device name", func() {
		net := "macvlan" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", "parent=lo", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]

		Expect(result.Driver).To(Equal("macvlan"))
		Expect(result.NetworkInterface).To(Equal("lo"))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "dhcp"))
		Expect(result.Subnets).To(HaveLen(0))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network exists", func() {
		net := "net" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "exists", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "exists", stringid.GenerateNonCryptoID()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman network create macvlan with network info and options", func() {
		net := "macvlan" + stringid.GenerateNonCryptoID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", "parent=lo", "-o", "mtu=1500", "--gateway", "192.168.1.254", "--subnet", "192.168.1.0/24", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		var results []types.Network
		err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(1))
		result := results[0]

		Expect(result.Options).To(HaveKeyWithValue("mtu", "1500"))
		Expect(result.Driver).To(Equal("macvlan"))
		Expect(result.NetworkInterface).To(Equal("lo"))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))

		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("192.168.1.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("192.168.1.254"))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network prune --filter", func() {
		// set custom cni directory to prevent flakes
		podmanTest.CNIConfigDir = tempdir
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		net1 := "macvlan" + stringid.GenerateNonCryptoID() + "net1"

		nc := podmanTest.Podman([]string{"network", "create", net1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net1)
		Expect(nc).Should(Exit(0))

		list := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).Should(HaveLen(2))

		Expect(list.OutputToStringArray()).Should(ContainElement(net1))
		Expect(list.OutputToStringArray()).Should(ContainElement("podman"))

		// -f needed only to skip y/n question
		prune := podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=50"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(2))

		Expect(listAgain.OutputToStringArray()).Should(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))

		// -f needed only to skip y/n question
		prune = podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=5000000000000"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(1))

		Expect(listAgain.OutputToStringArray()).ShouldNot(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))
	})

	It("podman network prune", func() {
		// set custom cni directory to prevent flakes
		podmanTest.CNIConfigDir = tempdir
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		// Create two networks
		// Check they are there
		// Run a container on one of them
		// Network Prune
		// Check that one has been pruned, other remains
		net := "macvlan" + stringid.GenerateNonCryptoID()
		net1 := net + "1"
		net2 := net + "2"
		nc := podmanTest.Podman([]string{"network", "create", net1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net1)
		Expect(nc).Should(Exit(0))

		nc2 := podmanTest.Podman([]string{"network", "create", net2})
		nc2.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net2)
		Expect(nc2).Should(Exit(0))

		list := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list.OutputToStringArray()).Should(HaveLen(3))

		Expect(list.OutputToStringArray()).Should(ContainElement(net1))
		Expect(list.OutputToStringArray()).Should(ContainElement(net2))
		Expect(list.OutputToStringArray()).Should(ContainElement("podman"))

		session := podmanTest.Podman([]string{"run", "-dt", "--net", net2, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		prune := podmanTest.Podman([]string{"network", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(2))

		Expect(listAgain.OutputToStringArray()).ShouldNot(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement(net2))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))
	})
})
