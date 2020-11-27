package integration

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containers/podman/v2/pkg/rootless"
	. "github.com/containers/podman/v2/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	It("podman network list --filter plugin and name", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=bridge", "--filter", "name=" + name})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter two names", func() {
		name1, path1 := generateNetworkConfig(podmanTest)
		defer removeConf(path1)

		name2, path2 := generateNetworkConfig(podmanTest)
		defer removeConf(path2)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "name=" + name1, "--filter", "name=" + name2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
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
		Expect(session.ExitCode()).To(BeZero())

		net2 := "labelnet" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", "--label", label1, "--label", label2, net2})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net2)
		Expect(session.ExitCode()).To(BeZero())

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1, "--filter", "label=" + label2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).ToNot(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))
	})

	It("podman network list --filter invalid value", func() {
		session := podmanTest.Podman([]string{"network", "ls", "--filter", "namr=ab"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring(`invalid filter "namr"`))
	})

	It("podman network list --filter failure", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "plugin=test"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.LineInOutputContains(name)).To(BeFalse())
	})

	rm_func := func(rm string) {
		It(fmt.Sprintf("podman network %s no args", rm), func() {
			session := podmanTest.Podman([]string{"network", rm})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).ToNot(BeZero())

		})

		It(fmt.Sprintf("podman network %s", rm), func() {
			name, path := generateNetworkConfig(podmanTest)
			defer removeConf(path)

			session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
			Expect(session.LineInOutputContains(name)).To(BeTrue())

			rm := podmanTest.Podman([]string{"network", rm, name})
			rm.WaitWithDefaultTimeout()
			Expect(rm.ExitCode()).To(BeZero())

			results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			results.WaitWithDefaultTimeout()
			Expect(results.ExitCode()).To(Equal(0))
			Expect(results.LineInOutputContains(name)).To(BeFalse())
		})
	}

	rm_func("rm")
	rm_func("remove")

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
		network := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.50.0/24", netName})
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
		net, ok := conData[0].NetworkSettings.Networks[netName]
		Expect(ok).To(BeTrue())
		Expect(net.NetworkID).To(Equal(netName))
		Expect(net.IPPrefixLen).To(Equal(24))
		Expect(strings.HasPrefix(net.IPAddress, "10.50.50.")).To(BeTrue())

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})

	It("podman inspect container two CNI networks (container not running)", func() {
		netName1 := "testNetThreeCNI1"
		network1 := podmanTest.Podman([]string{"network", "create", netName1})
		network1.WaitWithDefaultTimeout()
		Expect(network1.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName1)

		netName2 := "testNetThreeCNI2"
		network2 := podmanTest.Podman([]string{"network", "create", netName2})
		network2.WaitWithDefaultTimeout()
		Expect(network2.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName2)

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"create", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
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
		net2, ok := conData[0].NetworkSettings.Networks[netName2]
		Expect(ok).To(BeTrue())
		Expect(net2.NetworkID).To(Equal(netName2))

		// Necessary to ensure the CNI network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll.ExitCode()).To(BeZero())
	})

	It("podman inspect container two CNI networks", func() {
		netName1 := "testNetTwoCNI1"
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.0/25", netName1})
		network1.WaitWithDefaultTimeout()
		Expect(network1.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName1)

		netName2 := "testNetTwoCNI2"
		network2 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.128/26", netName2})
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
	It("podman network with multiple aliases", func() {
		Skip("Until DNSName is updated on our CI images")
		var worked bool
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		top := podmanTest.Podman([]string{"run", "-dt", "--name=web", "--network=" + netName, "--network-alias=web1", "--network-alias=web2", nginx})
		top.WaitWithDefaultTimeout()
		Expect(top.ExitCode()).To(BeZero())
		interval := time.Duration(250 * time.Millisecond)
		// Wait for the nginx service to be running
		for i := 0; i < 6; i++ {
			// Test curl against the container's name
			c1 := podmanTest.Podman([]string{"run", "--network=" + netName, nginx, "curl", "web"})
			c1.WaitWithDefaultTimeout()
			worked = Expect(c1.ExitCode()).To(BeZero())
			if worked {
				break
			}
			time.Sleep(interval)
			interval *= 2
		}
		Expect(worked).To(BeTrue())

		// Nginx is now running so no need to do a loop
		// Test against the first alias
		c2 := podmanTest.Podman([]string{"run", "--network=" + netName, nginx, "curl", "web1"})
		c2.WaitWithDefaultTimeout()
		Expect(c2.ExitCode()).To(BeZero())

		// Test against the second alias
		c3 := podmanTest.Podman([]string{"run", "--network=" + netName, nginx, "curl", "web2"})
		c3.WaitWithDefaultTimeout()
		Expect(c3.ExitCode()).To(BeZero())
	})

	It("bad network name in disconnect should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		dis := podmanTest.Podman([]string{"network", "disconnect", "foobar", "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())

	})

	It("bad container name in network disconnect should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())

	})

	It("podman network disconnect with invalid container state should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(BeZero())

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())
	})

	It("podman network disconnect", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(BeZero())

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec.ExitCode()).To(BeZero())

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).To(BeZero())

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec.ExitCode()).ToNot(BeZero())
	})

	It("bad network name in connect should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		dis := podmanTest.Podman([]string{"network", "connect", "foobar", "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())

	})

	It("bad container name in network connect should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		dis := podmanTest.Podman([]string{"network", "connect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())

	})

	It("podman connect on a container that already is connected to the network should error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(BeZero())

		con := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		con.WaitWithDefaultTimeout()
		Expect(con.ExitCode()).ToNot(BeZero())
	})

	It("podman network connect with invalid container state should result in error", func() {
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(BeZero())

		dis := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis.ExitCode()).ToNot(BeZero())
	})

	It("podman network connect", func() {
		SkipIfRemote("This requires a pending PR to be merged before it will work")
		SkipIfRootless("network connect and disconnect are only rootfull")
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(netName)

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr.ExitCode()).To(BeZero())

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec.ExitCode()).To(BeZero())

		// Create a second network
		newNetName := "aliasTest" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", newNetName})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(BeZero())
		defer podmanTest.removeCNINetwork(newNetName)

		connect := podmanTest.Podman([]string{"network", "connect", newNetName, "test"})
		connect.WaitWithDefaultTimeout()
		Expect(connect.ExitCode()).To(BeZero())

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec.ExitCode()).To(BeZero())
	})
})
