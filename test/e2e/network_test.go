package integration

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containers/podman/v3/pkg/rootless"
	. "github.com/containers/podman/v3/test/utils"
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

	It("podman network list", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list -q", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list --filter success", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=bridge"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains(name)).To(BeTrue())
	})

	It("podman network list --filter plugin and name", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=bridge", "--filter", "name=" + name})
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
		net1 := "labelnet" + stringid.GenerateRandomID()
		label1 := "testlabel1=abc"
		label2 := "abcdef"
		session := podmanTest.Podman([]string{"network", "create", "--label", label1, net1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net1)
		Expect(session).Should(Exit(0))

		net2 := "labelnet" + stringid.GenerateRandomID()
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
		net := "net" + stringid.GenerateRandomID()
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

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains(name)).To(BeFalse())
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
		expectedTable := "NETWORK ID NAME VERSION PLUGINS"
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
		Expect(session.ErrorToString()).To(ContainSubstring("no such network"))

		session = podmanTest.Podman([]string{"network", "rm", netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	rm_func := func(rm string) {
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
			Expect(session.LineInOutputContains(name)).To(BeTrue())

			rm := podmanTest.Podman([]string{"network", rm, name})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(Exit(0))

			results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			results.WaitWithDefaultTimeout()
			Expect(results).Should(Exit(0))
			Expect(results.LineInOutputContains(name)).To(BeFalse())
		})
	}

	rm_func("rm")
	rm_func("remove")

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
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "inspect", name, "--format", "{{.cniVersion}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.LineInOutputContains("0.3.0")).To(BeTrue())
	})

	It("podman inspect container single CNI network", func() {
		netName := "net-" + stringid.GenerateRandomID()
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
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(1))
		net, ok := conData[0].NetworkSettings.Networks[netName]
		Expect(ok).To(BeTrue())
		Expect(net.NetworkID).To(Equal(netName))
		Expect(net.IPPrefixLen).To(Equal(24))
		Expect(strings.HasPrefix(net.IPAddress, "10.50.50.")).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman inspect container two CNI networks (container not running)", func() {
		netName1 := "net1-" + stringid.GenerateRandomID()
		network1 := podmanTest.Podman([]string{"network", "create", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(network1).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateRandomID()
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
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman inspect container two CNI networks", func() {
		netName1 := "net1-" + stringid.GenerateRandomID()
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.0/25", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(network1).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateRandomID()
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
		Expect(len(conData)).To(Equal(1))
		Expect(len(conData[0].NetworkSettings.Networks)).To(Equal(2))
		net1, ok := conData[0].NetworkSettings.Networks[netName1]
		Expect(ok).To(BeTrue())
		Expect(net1.NetworkID).To(Equal(netName1))
		Expect(net1.IPPrefixLen).To(Equal(25))
		Expect(strings.HasPrefix(net1.IPAddress, "10.50.51.")).To(BeTrue())
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))
		Expect(net2.IPPrefixLen).To(Equal(26))
		Expect(strings.HasPrefix(net2.IPAddress, "10.50.51.")).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})

	It("podman network remove after disconnect when container initially created with the network", func() {
		SkipIfRootless("disconnect works only in non rootless container")

		container := "test"
		network := "foo"

		session := podmanTest.Podman([]string{"network", "create", network})
		session.WaitWithDefaultTimeout()
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
		netName := "net-" + stringid.GenerateRandomID()
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

		session = podmanTest.Podman([]string{"network", "rm", "--force", netName})
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
		netName1 := "net1-" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(netName1)
		Expect(session).Should(Exit(0))

		netName2 := "net2-" + stringid.GenerateRandomID()
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
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--macvlan", "lo", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network create/remove macvlan as driver (-d) no device name", func() {
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		out, err := inspect.jq(".[0].plugins[0].master")
		Expect(err).To(BeNil())
		Expect(out).To(Equal("\"\""))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network create/remove macvlan as driver (-d) with device name", func() {
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", "parent=lo", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		out, err := inspect.jq(".[0].plugins[0].master")
		Expect(err).To(BeNil())
		Expect(out).To(Equal(`"lo"`))

		ipamType, err := inspect.jq(".[0].plugins[0].ipam.type")
		Expect(err).To(BeNil())
		Expect(ipamType).To(Equal(`"dhcp"`))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network exists", func() {
		net := "net" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "exists", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"network", "exists", stringid.GenerateRandomID()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman network create macvlan with network info and options", func() {
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", "parent=lo", "-o", "mtu=1500", "--gateway", "192.168.1.254", "--subnet", "192.168.1.0/24", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(nc).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"network", "inspect", net})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		mtu, err := inspect.jq(".[0].plugins[0].mtu")
		Expect(err).To(BeNil())
		Expect(mtu).To(Equal("1500"))

		name, err := inspect.jq(".[0].plugins[0].type")
		Expect(err).To(BeNil())
		Expect(name).To(Equal(`"macvlan"`))

		netInt, err := inspect.jq(".[0].plugins[0].master")
		Expect(err).To(BeNil())
		Expect(netInt).To(Equal(`"lo"`))

		ipamType, err := inspect.jq(".[0].plugins[0].ipam.type")
		Expect(err).To(BeNil())
		Expect(ipamType).To(Equal(`"host-local"`))

		gw, err := inspect.jq(".[0].plugins[0].ipam.ranges[0][0].gateway")
		Expect(err).To(BeNil())
		Expect(gw).To(Equal(`"192.168.1.254"`))

		subnet, err := inspect.jq(".[0].plugins[0].ipam.ranges[0][0].subnet")
		Expect(err).To(BeNil())
		Expect(subnet).To(Equal(`"192.168.1.0/24"`))

		routes, err := inspect.jq(".[0].plugins[0].ipam.routes[0].dst")
		Expect(err).To(BeNil())
		Expect(routes).To(Equal(`"0.0.0.0/0"`))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(Exit(0))
	})

	It("podman network prune --filter", func() {
		net1 := "macvlan" + stringid.GenerateRandomID() + "net1"

		nc := podmanTest.Podman([]string{"network", "create", net1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net1)
		Expect(nc).Should(Exit(0))

		list := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))

		Expect(StringInSlice(net1, list.OutputToStringArray())).To(BeTrue())
		if !isRootless() {
			Expect(StringInSlice("podman", list.OutputToStringArray())).To(BeTrue())
		}

		// -f needed only to skip y/n question
		prune := podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=50"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))

		Expect(StringInSlice(net1, listAgain.OutputToStringArray())).To(BeTrue())
		if !isRootless() {
			Expect(StringInSlice("podman", list.OutputToStringArray())).To(BeTrue())
		}

		// -f needed only to skip y/n question
		prune = podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=5000000000000"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))

		Expect(StringInSlice(net1, listAgain.OutputToStringArray())).To(BeFalse())
		if !isRootless() {
			Expect(StringInSlice("podman", list.OutputToStringArray())).To(BeTrue())
		}
	})

	It("podman network prune", func() {
		// Create two networks
		// Check they are there
		// Run a container on one of them
		// Network Prune
		// Check that one has been pruned, other remains
		net := "macvlan" + stringid.GenerateRandomID()
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
		Expect(list).Should(Exit(0))

		Expect(StringInSlice(net1, list.OutputToStringArray())).To(BeTrue())
		Expect(StringInSlice(net2, list.OutputToStringArray())).To(BeTrue())
		if !isRootless() {
			Expect(StringInSlice("podman", list.OutputToStringArray())).To(BeTrue())
		}

		session := podmanTest.Podman([]string{"run", "-dt", "--net", net2, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		prune := podmanTest.Podman([]string{"network", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(Exit(0))

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(Exit(0))

		Expect(StringInSlice(net1, listAgain.OutputToStringArray())).To(BeFalse())
		Expect(StringInSlice(net2, listAgain.OutputToStringArray())).To(BeTrue())
		// Make sure default network 'podman' still exists
		if !isRootless() {
			Expect(StringInSlice("podman", list.OutputToStringArray())).To(BeTrue())
		}

	})
})
