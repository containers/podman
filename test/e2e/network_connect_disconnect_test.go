package integration

import (
	"os"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Podman network connect and disconnect", func() {
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

	It("bad network name in disconnect should result in error", func() {
		dis := podmanTest.Podman([]string{"network", "disconnect", "foobar", "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError())
	})

	It("bad container name in network disconnect should result in error", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError())
	})

	It("network disconnect with net mode slirp4netns should result in error", func() {
		netName := "slirp" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"create", "--name", "test", "--network", "slirp4netns", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		con := podmanTest.Podman([]string{"network", "disconnect", netName, "test"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(ExitWithError())
		Expect(con.ErrorToString()).To(ContainSubstring(`"slirp4netns" is not supported: invalid network mode`))
	})

	It("podman network disconnect", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		gw := podmanTest.Podman([]string{"network", "inspect", netName, "--format", "{{(index .Subnets 0).Gateway}}"})
		gw.WaitWithDefaultTimeout()
		Expect(gw).Should(Exit(0))
		ns := gw.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		exec2 := podmanTest.Podman([]string{"exec", "-it", "test", "cat", "/etc/resolv.conf"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(Exit(0))
		Expect(strings.Contains(exec2.OutputToString(), ns)).To(BeTrue())

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(Exit(0))
		Expect(dis.ErrorToString()).Should(Equal(""))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("0"))

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitWithError())

		exec3 := podmanTest.Podman([]string{"exec", "-it", "test", "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(Exit(0))
		Expect(strings.Contains(exec3.OutputToString(), ns)).To(BeFalse())

		// make sure stats still works https://github.com/containers/podman/issues/13824
		stats := podmanTest.Podman([]string{"stats", "test", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))
	})

	It("bad network name in connect should result in error", func() {
		dis := podmanTest.Podman([]string{"network", "connect", "foobar", "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError())
	})

	It("bad container name in network connect should result in error", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		dis := podmanTest.Podman([]string{"network", "connect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError())
	})

	It("network connect with net mode slirp4netns should result in error", func() {
		netName := "slirp" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"create", "--name", "test", "--network", "slirp4netns", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		con := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(ExitWithError())
		Expect(con.ErrorToString()).To(ContainSubstring(`"slirp4netns" is not supported: invalid network mode`))
	})

	It("podman connect on a container that already is connected to the network should error", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))
		cid := ctr.OutputToString()

		// network alias container short id is always added and shown in inspect
		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{(index .NetworkSettings.Networks \"" + netName + "\").Aliases}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("[" + cid[0:12] + "]"))

		con := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(ExitWithError())
	})

	It("podman network connect", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))
		cid := ctr.OutputToString()

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		// Create a second network
		newNetName := "aliasTest" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", newNetName, "--subnet", "10.11.100.0/24"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(newNetName)

		gw := podmanTest.Podman([]string{"network", "inspect", newNetName, "--format", "{{(index .Subnets 0).Gateway}}"})
		gw.WaitWithDefaultTimeout()
		Expect(gw).Should(Exit(0))
		ns := gw.OutputToString()

		exec2 := podmanTest.Podman([]string{"exec", "-it", "test", "cat", "/etc/resolv.conf"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(Exit(0))
		Expect(strings.Contains(exec2.OutputToString(), ns)).To(BeFalse())

		ip := "10.11.100.99"
		mac := "44:11:44:11:44:11"
		connect := podmanTest.Podman([]string{"network", "connect", "--ip", ip, "--mac-address", mac, newNetName, "test"})
		connect.WaitWithDefaultTimeout()
		Expect(connect).Should(Exit(0))
		Expect(connect.ErrorToString()).Should(Equal(""))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("2"))

		// network alias container short id is always added and shown in inspect
		inspect = podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{(index .NetworkSettings.Networks \"" + newNetName + "\").Aliases}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("[" + cid[0:12] + "]"))

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		Expect(exec.OutputToString()).Should(ContainSubstring(ip))
		Expect(exec.OutputToString()).Should(ContainSubstring(mac))

		exec3 := podmanTest.Podman([]string{"exec", "-it", "test", "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(Exit(0))
		Expect(strings.Contains(exec3.OutputToString(), ns)).To(BeTrue())

		// make sure stats works https://github.com/containers/podman/issues/13824
		stats := podmanTest.Podman([]string{"stats", "test", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(Exit(0))

		// make sure no logrus errors are shown https://github.com/containers/podman/issues/9602
		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(Exit(0))
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman network connect when not running", func() {
		netName1 := "connect1" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName1)

		netName2 := "connect2" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", netName2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName2)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName1, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		dis := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("2"))

		start := podmanTest.Podman([]string{"start", "test"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
	})

	It("podman network connect and run with network ID", func() {
		netName := "ID" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		netID := session.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netID, "--network-alias", "somealias", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		// Create a second network
		newNetName := "ID2" + stringid.GenerateNonCryptoID()
		session = podmanTest.Podman([]string{"network", "create", newNetName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(newNetName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + newNetName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		newNetID := session.OutputToString()

		connect := podmanTest.Podman([]string{"network", "connect", "--alias", "secondalias", newNetID, "test"})
		connect.WaitWithDefaultTimeout()
		Expect(connect).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{.NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(netName))
		Expect(inspect.OutputToString()).To(ContainSubstring(newNetName))

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
	})

	It("podman network disconnect when not running", func() {
		netName1 := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName1)

		netName2 := "aliasTest" + stringid.GenerateNonCryptoID()
		session2 := podmanTest.Podman([]string{"network", "create", netName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		defer podmanTest.removeNetwork(netName2)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName1 + "," + netName2, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		dis := podmanTest.Podman([]string{"network", "disconnect", netName1, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("1"))

		start := podmanTest.Podman([]string{"start", "test"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()

		// because the network interface order is not guaranteed to be the same we have to check both eth0 and eth1
		// if eth0 did not exists eth1 has to exists
		var exitMatcher types.GomegaMatcher = ExitWithError()
		if exec.ExitCode() > 0 {
			exitMatcher = Exit(0)
		}

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(exitMatcher)
	})

	It("podman network disconnect and run with network ID", func() {
		netName := "aliasTest" + stringid.GenerateNonCryptoID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		netID := session.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netID, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		dis := podmanTest.Podman([]string{"network", "disconnect", netID, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("0"))

		exec = podmanTest.Podman([]string{"exec", "-it", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitWithError())
	})
})
