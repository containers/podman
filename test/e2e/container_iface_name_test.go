package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

func createNetworkDevice(name string) {
        session := SystemExec("ip", []string{"link", "add", name, "type", "bridge"})
        session.WaitWithDefaultTimeout()
}

func removeNetworkDevice(name string) {
        session := SystemExec("ip", []string{"link", "delete", name})
        session.WaitWithDefaultTimeout()
}

func createContainersConfFileWithDeviceIfaceName(pTest *PodmanTestIntegration) {
	configPath := filepath.Join(pTest.TempDir, "containers.conf")
	containersConf := []byte(fmt.Sprintf("[containers]\ninterface_name = \"device\"\n"))
	err := os.WriteFile(configPath, containersConf, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	// Set custom containers.conf file
	os.Setenv("CONTAINERS_CONF", configPath)
	if IsRemote() {
		pTest.RestartRemoteService()
	}
}


var _ = Describe("Podman container interface name", func() {

	It("podman container interface name with default scheme for bridge network", func() {
		netName1 := "bridge" + stringid.GenerateRandomID()
		netName2 := "bridge" + stringid.GenerateRandomID()

		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		// Inspect the network configuration
		nic1 := podmanTest.Podman([]string{"network", "inspect", netName1, "--format", "{{.NetworkInterface}}"})
		nic1.WaitWithDefaultTimeout()
		Expect(nic1).Should(ExitCleanly())

		// Once a container executes a new network, the nic will be
		// created. We should clean those up best we can
		defer removeNetworkDevice(nic1.OutputToString())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		// Inspect the network configuration
		nic2 := podmanTest.Podman([]string{"network", "inspect", netName2, "--format", "{{.NetworkInterface}}"})
		nic2.WaitWithDefaultTimeout()
		Expect(nic2).Should(ExitCleanly())

		// Once a container executes a new network, the nic will be
		// created. We should clean those up best we can
		defer removeNetworkDevice(nic2.OutputToString())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman container interface name with default scheme for macvlan network with no parent", func() {
		netName1 := "macvlan" + stringid.GenerateRandomID()
		netName2 := "macvlan" + stringid.GenerateRandomID()

		// There is no nic created by the macvlan driver.
		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman container interface name with default scheme for macvlan network with parent", func() {
		// Create a nic to be used as a parent for macvlan network.
		nicName1 := "br" + stringid.GenerateRandomID()
		nicName2 := "br" + stringid.GenerateRandomID()

		netName1 := "macvlan" + stringid.GenerateRandomID()
		netName2 := "macvlan" + stringid.GenerateRandomID()

		parent1 := "parent=" + nicName1
		parent2 := "parent=" + nicName2

		defer removeNetworkDevice(nicName1)
		createNetworkDevice(nicName1)

		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", parent1, netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer removeNetworkDevice(nicName2)
		createNetworkDevice(nicName2)

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", parent2, netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman container interface name with host scheme for bridge network", func() {
		createContainersConfFileWithDeviceIfaceName(podmanTest)

		netName1 := "bridge" + stringid.GenerateRandomID()
		netName2 := "bridge" + stringid.GenerateRandomID()

		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		// Inspect the network configuration
		nic1 := podmanTest.Podman([]string{"network", "inspect", netName1, "--format", "{{.NetworkInterface}}"})
		nic1.WaitWithDefaultTimeout()
		Expect(nic1).Should(ExitCleanly())

		// Once a container executes a new network, the nic will be
		// created. We should clean those up best we can
		defer removeNetworkDevice(nic1.OutputToString())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nic1.OutputToString()})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		// Inspect the network configuration
		nic2 := podmanTest.Podman([]string{"network", "inspect", netName2, "--format", "{{.NetworkInterface}}"})
		nic2.WaitWithDefaultTimeout()
		Expect(nic2).Should(ExitCleanly())

		// Once a container executes a new network, the nic will be
		// created. We should clean those up best we can
		defer removeNetworkDevice(nic2.OutputToString())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nic2.OutputToString()})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman container interface name with host scheme for macvlan network with no parent", func() {
		createContainersConfFileWithDeviceIfaceName(podmanTest)

		netName1 := "macvlan" + stringid.GenerateRandomID()
		netName2 := "macvlan" + stringid.GenerateRandomID()

		// There is no nic created by the macvlan driver.
		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman container interface name with host scheme for macvlan network with parent", func() {
		createContainersConfFileWithDeviceIfaceName(podmanTest)

		// Create a nic to be used as a parent for macvlan network.
		nicName1 := "br" + stringid.GenerateRandomID()
		nicName2 := "br" + stringid.GenerateRandomID()

		netName1 := "macvlan" + stringid.GenerateRandomID()
		netName2 := "macvlan" + stringid.GenerateRandomID()

		parent1 := "parent=" + nicName1
		parent2 := "parent=" + nicName2

		defer removeNetworkDevice(nicName1)
		createNetworkDevice(nicName1)

		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", parent1, netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		defer podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		ctr := podmanTest.Podman([]string{"run", "-dt", "--network", netName1, "--name", "test", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nicName1})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		defer removeNetworkDevice(nicName2)
		createNetworkDevice(nicName2)

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", parent2, netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		conn.WaitWithDefaultTimeout()
		Expect(conn).Should(ExitCleanly())
		Expect(conn.ErrorToString()).Should(Equal(""))

		exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nicName2})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

})
