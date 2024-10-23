//go:build linux

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v5/pkg/domain/entities"
	. "github.com/containers/podman/v5/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/vishvananda/netlink"
)

var _ = Describe("Podman run networking", func() {

	hostname, _ := os.Hostname()

	It("podman verify network scoped DNS server and also verify updating network dns server", func() {
		// Following test is only functional with netavark and aardvark
		SkipIfCNI(podmanTest)
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net, "--dns", "1.1.1.1"})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "inspect", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())
		var results []entities.NetworkInspectReport
		err := json.Unmarshal([]byte(session.OutputToString()), &results)
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Subnets).To(HaveLen(1))
		aardvarkDNSGateway := result.Subnets[0].Gateway.String()
		Expect(result.NetworkDNSServers).To(Equal([]string{"1.1.1.1"}))

		session = podmanTest.Podman([]string{"run", "-d", "--name", "con1", "--network", net, "busybox", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Non-authoritative answer: Name: google.com Address:"))

		// Update to a bad DNS Server
		session = podmanTest.Podman([]string{"network", "update", net, "--dns-add", "127.0.0.255"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Remove good DNS server
		session = podmanTest.Podman([]string{"network", "update", net, "--dns-drop=1.1.1.1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(ContainSubstring(";; connection timed out; no servers could be reached"))
	})

	It("podman network dns multiple servers", func() {
		// Following test is only functional with netavark and aardvark
		SkipIfCNI(podmanTest)
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net, "--dns", "1.1.1.1,8.8.8.8", "--dns", "8.4.4.8"})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "inspect", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())
		var results []entities.NetworkInspectReport
		err := json.Unmarshal([]byte(session.OutputToString()), &results)
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Subnets).To(HaveLen(1))
		aardvarkDNSGateway := result.Subnets[0].Gateway.String()
		Expect(result.NetworkDNSServers).To(Equal([]string{"1.1.1.1", "8.8.8.8", "8.4.4.8"}))

		session = podmanTest.Podman([]string{"run", "-d", "--name", "con1", "--network", net, "busybox", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Non-authoritative answer: Name: google.com Address:"))

		// Update DNS server
		session = podmanTest.Podman([]string{"network", "update", net, "--dns-drop=1.1.1.1,8.8.8.8",
			"--dns-drop", "8.4.4.8", "--dns-add", "127.0.0.253,127.0.0.254", "--dns-add", "127.0.0.255"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "inspect", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())
		err = json.Unmarshal([]byte(session.OutputToString()), &results)
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))
		Expect(results[0].NetworkDNSServers).To(Equal([]string{"127.0.0.253", "127.0.0.254", "127.0.0.255"}))

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(ContainSubstring(";; connection timed out; no servers could be reached"))
	})

	It("podman run network connection with default bridge", func() {
		session := podmanTest.RunContainerWithNetworkTest("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with host", func() {
		session := podmanTest.RunContainerWithNetworkTest("host")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with default", func() {
		session := podmanTest.RunContainerWithNetworkTest("default")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with none", func() {
		session := podmanTest.RunContainerWithNetworkTest("none")
		session.WaitWithDefaultTimeout()
		if _, found := os.LookupEnv("http_proxy"); found {
			Expect(session).Should(ExitWithError(5, "Could not resolve proxy:"))
		} else {
			Expect(session).Should(ExitWithError(6, "Could not resolve host: www.redhat.com"))
		}
	})

	It("podman run network connection with private", func() {
		session := podmanTest.RunContainerWithNetworkTest("private")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman verify resolv.conf with --dns + --network", func() {
		// Following test is only functional with netavark and aardvark
		// since new behaviour depends upon output from of statusBlock
		SkipIfCNI(podmanTest)
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con1", "--dns", "1.1.1.1", "--network", net, ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Must not contain custom dns server in containers
		// `/etc/resolv.conf` since custom dns-server is
		// already expected to be present and processed by
		// Podman's DNS resolver i.e ( aarvark-dns or dnsname ).
		Expect(session.OutputToString()).ToNot(ContainSubstring("nameserver 1.1.1.1"))
		// But /etc/resolve.conf must contain other nameserver
		// i.e dns server configured for network.
		Expect(session.OutputToString()).To(ContainSubstring("nameserver"))

		session = podmanTest.Podman([]string{"run", "--name", "con2", "--dns", "1.1.1.1", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// All the networks being used by following container
		// don't have dns_enabled in such scenario `/etc/resolv.conf`
		// must contain nameserver which were specified via `--dns`.
		Expect(session.OutputToString()).To(ContainSubstring("nameserver 1.1.1.1"))
	})

	It("podman run -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80-82 -p 8090:8090", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-82", "-p", "8090:8090", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0].HostPort).To(Not(Equal("82")))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0]).To(HaveField("HostPort", "8090"))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80-81 -p 8180-8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-81", "-p", "8180-8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0].HostPort).To(Not(Equal("8180")))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80 -p 8280-8282:8280-8282", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "-p", "8280-8282:8280-8282", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0]).To(HaveField("HostPort", "8280"))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0]).To(HaveField("HostPort", "8281"))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0]).To(HaveField("HostPort", "8282"))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 8380:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8380:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8380"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 8480:80/TCP", func() {
		name := "testctr"
		// "TCP" in upper characters
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8480:80/TCP", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		// "tcp" in lower characters
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8480"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 127.0.0.1:8580:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8580:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8580"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p 127.0.0.1:8680:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8680:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostPort", "8680"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p [::1]:8780:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8780:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostPort", "8780"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "::1"))
	})

	It("podman run -p [::1]:8880:80/tcp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8880:80/tcp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8880"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "::1"))
	})

	It("podman run --expose 80 -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "80", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose 80/udp -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "80/udp", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose port range", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "1000-9999", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see anything extra
		Expect(session.OutputToString()).To(Equal("1000-9999/tcp"))

		name := "testctr"
		session = podmanTest.Podman([]string{"run", "-d", "--expose", "222-223", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(2))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose 80 -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--expose", "80", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		hostPort := inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort
		Expect(hostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see the extra ", 80/tcp" from the exposed port
		Expect(session.OutputToString()).To(Equal("0.0.0.0:" + hostPort + "->80/tcp"))
	})

	It("podman run --publish-all with EXPOSE port ranges in Dockerfile", func() {
		// Test port ranges, range with protocol and with an overlapping port
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := fmt.Sprintf(`FROM %s
EXPOSE 2002
EXPOSE 2001-2003
EXPOSE 2004-2005/tcp`, ALPINE)
		imageName := "testimg"
		podmanTest.BuildImage(dockerfile, imageName, "false")

		// Verify that the buildah is just passing through the EXPOSE keys
		inspect := podmanTest.Podman([]string{"inspect", imageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		image := inspect.InspectImageJSON()
		Expect(image).To(HaveLen(1))
		Expect(image[0].Config.ExposedPorts).To(HaveLen(3))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2002/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2001-2003/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2004-2005/tcp"))

		session := podmanTest.Podman([]string{"create", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see anything extra
		Expect(session.OutputToString()).To(Equal("2001-2005/tcp"))

		containerName := "testcontainer"
		session = podmanTest.Podman([]string{"create", "--publish-all", "--name", containerName, imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(containerName)
		Expect(inspectOut).To(HaveLen(1))

		// Inspect the network settings with available ports to be mapped to the host
		// Don't need to verity HostConfig.PortBindings since we used --publish-all
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(5))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2001/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2002/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2003/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2004/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2005/tcp"))
		Expect(inspectOut[0].HostConfig.PublishAllPorts).To(BeTrue())
	})

	It("podman run --net=host --expose includes ports in inspect output", func() {
		containerName := "testctr"
		session := podmanTest.Podman([]string{"run", "--net=host", "--name", containerName, "-d", "--expose", "8080/tcp", NGINX_IMAGE, "sleep", "+inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(containerName)
		Expect(inspectOut).To(HaveLen(1))

		// Ports is empty. ExposedPorts is not.
		Expect(inspectOut[0].NetworkSettings.Ports).To(BeEmpty())

		// 80 from the image, 8080 from the expose
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveLen(2))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("80/tcp"))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("8080/tcp"))
	})

	It("podman run --net=container --expose exposed port from own container", func() {
		ctr1 := "test1"
		session1 := podmanTest.Podman([]string{"run", "-d", "--name", ctr1, "--expose", "8080/tcp", ALPINE, "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		ctr2 := "test2"
		session2 := podmanTest.Podman([]string{"run", "-d", "--name", ctr2, "--net", fmt.Sprintf("container:%s", ctr1), "--expose", "8090/tcp", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(ctr2)
		Expect(inspectOut).To(HaveLen(1))
		// Ports will not be populated. ExposedPorts will be.
		Expect(inspectOut[0].NetworkSettings.Ports).To(BeEmpty())
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveLen(1))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("8090/tcp"))
	})

	It("podman run -p 127.0.0.1::8980/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1::8980/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0].HostPort).To(Not(Equal("8980")))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p :8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", ":8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p xxx:8080 -p yyy:8080", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "4444:8080", "-p", "5555:8080", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8080/tcp"]).To(HaveLen(2))

		hp1 := inspectOut[0].NetworkSettings.Ports["8080/tcp"][0].HostPort
		hp2 := inspectOut[0].NetworkSettings.Ports["8080/tcp"][1].HostPort

		// We can't guarantee order
		Expect((hp1 == "4444" && hp2 == "5555") || (hp1 == "5555" && hp2 == "4444")).To(BeTrue())
	})

	It("podman run -p 0.0.0.0:9280:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "0.0.0.0:9280:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "9280"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run slirp4netns verify net.ipv6.conf.default.accept_dad=0", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:enable_ipv6=true", ALPINE, "ip", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// check the ipv6 setup id done without delay (https://github.com/containers/podman/issues/11062)
		Expect(session.OutputToString()).To(ContainSubstring("inet6 fd00::"))

		const ipv6ConfDefaultAcceptDadSysctl = "/proc/sys/net/ipv6/conf/all/accept_dad"

		cat := SystemExec("cat", []string{ipv6ConfDefaultAcceptDadSysctl})
		cat.WaitWithDefaultTimeout()
		Expect(cat).Should(ExitCleanly())
		sysctlValue := cat.OutputToString()

		session = podmanTest.Podman([]string{"run", "--network", "slirp4netns:enable_ipv6=true", ALPINE, "cat", ipv6ConfDefaultAcceptDadSysctl})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(sysctlValue))
	})

	It("podman run network expose host port 8080 to container port 8000 using invalid port handler", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:port_handler=invalid", "-dt", "-p", "8080:8000", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(126, `unknown port_handler for slirp4netns: "invalid"`))
	})

	It("podman run slirp4netns network with host loopback", func() {
		session := podmanTest.Podman([]string{"run", "--cap-add", "net_raw", "--network", "slirp4netns:allow_host_loopback=true", ALPINE, "ping", "-c1", "10.0.2.2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run slirp4netns network with mtu", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:mtu=9000", ALPINE, "ip", "addr"})
		session.Wait(30)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mtu 9000"))
	})

	It("podman run slirp4netns network with different cidr", func() {
		slirp4netnsHelp := SystemExec("slirp4netns", []string{"--help"})
		Expect(slirp4netnsHelp).Should(ExitCleanly())

		networkConfiguration := "slirp4netns:cidr=192.168.0.0/24,allow_host_loopback=true"
		session := podmanTest.Podman([]string{"run", "--cap-add", "net_raw", "--network", networkConfiguration, ALPINE, "ping", "-c1", "192.168.0.2"})
		session.Wait(30)

		if strings.Contains(slirp4netnsHelp.OutputToString(), "cidr") {
			Expect(session).Should(ExitCleanly())
		} else {
			Expect(session).To(ExitWithError(125, "cidr not supported"))
		}
	})

	for _, local := range []bool{true, false} {
		testName := "HostIP"
		if local {
			testName = "127.0.0.1"
		}
		It(fmt.Sprintf("podman run network slirp4netns bind to %s", testName), func() {
			ip := "127.0.0.1"
			if !local {
				// Determine our likeliest outgoing IP address
				conn, err := net.Dial("udp", "8.8.8.8:80")
				Expect(err).ToNot(HaveOccurred())

				defer conn.Close()
				ip = conn.LocalAddr().(*net.UDPAddr).IP.String()
			}
			port := strconv.Itoa(GetPort())

			networkConfiguration := fmt.Sprintf("slirp4netns:outbound_addr=%s,allow_host_loopback=true", ip)

			listener, err := net.Listen("tcp", ":"+port)
			Expect(err).ToNot(HaveOccurred())
			defer listener.Close()

			msg := RandomString(10)
			wg := &sync.WaitGroup{}
			wg.Add(1)
			// now use a new goroutine to start accepting connection in the background and make the checks there
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				conn, err := listener.Accept()
				Expect(err).ToNot(HaveOccurred(), "accept new connection")
				defer conn.Close()
				addr := conn.RemoteAddr()
				// addr will be in the form ip:port, we don't care about the port as it is random
				Expect(addr.String()).To(HavePrefix(ip+":"), "remote address")
				gotBytes, err := io.ReadAll(conn)
				Expect(err).ToNot(HaveOccurred(), "read from connection")
				Expect(string(gotBytes)).To(Equal(msg), "received correct message from container")
			}()

			session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, ALPINE, "sh", "-c", "echo -n " + msg + " | nc -w 30 10.0.2.2 " + port})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// explicitly close the socket here before we wait to unlock Accept() calls in case of hangs
			listener.Close()
			// wait for the checks in the goroutine to be done
			wg.Wait()
		})
	}

	It("podman run network expose ports in image metadata", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "-t", "-P", NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		results := podmanTest.Podman([]string{"inspect", "test"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToString()).To(ContainSubstring(`"80/tcp":`))
	})

	It("podman run network expose duplicate host port results in error", func() {
		port := "8190" // Make sure this isn't used anywhere else

		session := podmanTest.Podman([]string{"run", "--name", "test", "-dt", "-p", port, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		containerConfig := inspect.InspectContainerToJSON()
		Expect(containerConfig[0].NetworkSettings.Ports).To(Not(BeNil()))
		Expect(containerConfig[0].NetworkSettings.Ports).To(HaveKeyWithValue(port+"/tcp", Not(BeNil())))
		Expect(containerConfig[0].NetworkSettings.Ports[port+"/tcp"][0].HostPort).ToNot(Equal(port))
	})

	It("podman run forward sctp protocol", func() {
		SkipIfRootless("sctp protocol only works as root")
		session := podmanTest.Podman([]string{"--log-level=info", "run", "--name=test", "-p", "80/sctp", "-p", "81/sctp", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// we can only check logrus on local podman
		if !IsRemote() {
			// check that the info message for sctp protocol is only displayed once
			Expect(strings.Count(session.ErrorToString(), "Port reservation for SCTP is not supported")).To(Equal(1), "`Port reservation for SCTP is not supported` is not displayed exactly one time in the logrus logs")
		}
		results := podmanTest.Podman([]string{"inspect", "test"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToString()).To(ContainSubstring(`"80/sctp":`))
		Expect(results.OutputToString()).To(ContainSubstring(`"81/sctp":`))
	})

	It("podman run hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring(hostname)))
	})

	It("podman run --net host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})
	It("podman run --net host --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})
	It("podman run --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run --net host --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --net container: and --uts container:", func() {
		ctrName := "ctrToJoin"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--uts=container:" + ctrName, ALPINE, "true"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())
	})

	It("podman run --net container: and --add-host should fail", func() {
		ctrName := "ctrToJoin"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--add-host", "host1:127.0.0.1", ALPINE, "true"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitWithError(125, "cannot set extra host entries when the container is joined to another containers network namespace: invalid configuration"))
	})

	It("podman run --net container: copies hosts and resolv", func() {
		ctrName := "ctr1"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		// Exec in and modify /etc/resolv.conf and /etc/hosts
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo nameserver 192.0.2.1 > /etc/resolv.conf"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo 192.0.2.2 test1 > /etc/hosts"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--name", ctrName2, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		exec3 := podmanTest.Podman([]string{"exec", ctrName2, "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).To(ContainSubstring("nameserver 192.0.2.1"))

		exec4 := podmanTest.Podman([]string{"exec", ctrName2, "cat", "/etc/hosts"})
		exec4.WaitWithDefaultTimeout()
		Expect(exec4).Should(ExitCleanly())
		Expect(exec4.OutputToString()).To(ContainSubstring("192.0.2.2 test1"))
	})

	It("podman run /etc/hosts contains --hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --uidmap /etc/hosts contains --hostname", func() {
		SkipIfRootless("uidmap population of cninetworks not supported for rootless users")
		session := podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", "-v", "/etc/hosts:/etc/hosts", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
	})

	It("podman run network in user created network namespace", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX := SystemExec("ip", []string{"netns", "add", "xxx"})
		Expect(addXXX).Should(ExitCleanly())
		defer func() {
			delXXX := SystemExec("ip", []string{"netns", "delete", "xxx"})
			Expect(delXXX).Should(ExitCleanly())
		}()

		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxx", ALPINE, "wget", "www.redhat.com"})
		session.Wait(90)
		Expect(session).Should(ExitCleanly())
	})

	It("podman run n user created network namespace with resolv.conf", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX2 := SystemExec("ip", []string{"netns", "add", "xxx2"})
		Expect(addXXX2).Should(ExitCleanly())
		defer func() {
			delXXX2 := SystemExec("ip", []string{"netns", "delete", "xxx2"})
			Expect(delXXX2).Should(ExitCleanly())
		}()

		mdXXX2 := SystemExec("mkdir", []string{"-p", "/etc/netns/xxx2"})
		Expect(mdXXX2).Should(ExitCleanly())
		defer os.RemoveAll("/etc/netns/xxx2")

		nsXXX2 := SystemExec("bash", []string{"-c", "echo nameserver 11.11.11.11 > /etc/netns/xxx2/resolv.conf"})
		Expect(nsXXX2).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--net", "ns:/run/netns/xxx2", ALPINE, "cat", "/etc/resolv.conf"})
		session.Wait(90)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("11.11.11.11"))
	})

	addAddr := func(cidr string, containerInterface netlink.Link) error {
		_, ipnet, err := net.ParseCIDR(cidr)
		Expect(err).ToNot(HaveOccurred())
		addr := &netlink.Addr{IPNet: ipnet, Label: ""}
		if err := netlink.AddrAdd(containerInterface, addr); err != nil && err != syscall.EEXIST {
			return err
		}
		return nil
	}

	loopbackup := func() {
		lo, err := netlink.LinkByName("lo")
		Expect(err).ToNot(HaveOccurred())
		err = netlink.LinkSetUp(lo)
		Expect(err).ToNot(HaveOccurred())
	}

	linkup := func(name string, mac string, addresses []string) {
		linkAttr := netlink.NewLinkAttrs()
		linkAttr.Name = name
		m, err := net.ParseMAC(mac)
		Expect(err).ToNot(HaveOccurred())
		linkAttr.HardwareAddr = m
		eth := &netlink.Dummy{LinkAttrs: linkAttr}
		err = netlink.LinkAdd(eth)
		Expect(err).ToNot(HaveOccurred())
		err = netlink.LinkSetUp(eth)
		Expect(err).ToNot(HaveOccurred())
		for _, address := range addresses {
			err := addAddr(address, eth)
			Expect(err).ToNot(HaveOccurred())
		}
	}

	routeAdd := func(gateway string) {
		gw := net.ParseIP(gateway)
		route := &netlink.Route{Dst: nil, Gw: gw}
		err = netlink.RouteAdd(route)
		Expect(err).ToNot(HaveOccurred())
	}

	setupNetworkNs := func(networkNSName string) {
		_ = ns.WithNetNSPath("/run/netns/"+networkNSName, func(_ ns.NetNS) error {
			loopbackup()
			linkup("eth0", "46:7f:45:6e:4f:c8", []string{"10.25.40.0/24", "fd04:3e42:4a4e:3381::/64"})
			linkup("eth1", "56:6e:35:5d:3e:a8", []string{"10.88.0.0/16"})

			routeAdd("10.25.40.0")
			return nil
		})
	}

	checkNetworkNsInspect := func(name string) {
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPAddress", "10.25.40.0"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPPrefixLen", 24))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses[0]).To(HaveField("Addr", "10.88.0.0"))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses[0]).To(HaveField("PrefixLength", 16))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("GlobalIPv6Address", "fd04:3e42:4a4e:3381::"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("GlobalIPv6PrefixLen", 64))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPv6Addresses).To(BeEmpty())
		Expect(inspectOut[0].NetworkSettings).To(HaveField("MacAddress", "46:7f:45:6e:4f:c8"))
		Expect(inspectOut[0].NetworkSettings.AdditionalMacAddresses).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.AdditionalMacAddresses[0]).To(Equal("56:6e:35:5d:3e:a8"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("Gateway", "10.25.40.0"))

	}

	It("podman run network inspect fails gracefully on non-reachable network ns", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())

		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "-d", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		// delete the named network ns before inspect
		delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
		Expect(delNetworkNamespace).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPAddress", ""))
		Expect(inspectOut[0].NetworkSettings.Networks).To(BeEmpty())
	})

	It("podman inspect can handle joined network ns with multiple interfaces", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())
		defer func() {
			delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
			Expect(delNetworkNamespace).Should(ExitCleanly())
		}()
		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"container", "rm", name})
		session.WaitWithDefaultTimeout()

		// no network teardown should touch joined network ns interfaces
		session = podmanTest.Podman([]string{"run", "-d", "--replace", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)
	})

	It("podman do not tamper with joined network ns interfaces", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())
		defer func() {
			delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
			Expect(delNetworkNamespace).Should(ExitCleanly())
		}()

		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)

		name = RandomString(12)
		session = podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)

		// delete container, the network inspect should not change
		session = podmanTest.Podman([]string{"container", "rm", name})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "--replace", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)
	})

	It("podman run network in bogus user created network namespace", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxy", ALPINE, "wget", "www.redhat.com"})
		session.Wait(90)
		Expect(session).To(ExitWithError(125, "faccessat /run/netns/xxy: no such file or directory"))
	})

	It("podman run in custom CNI network with --static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.30.128"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.30.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		run := podmanTest.Podman([]string{"run", "--rm", "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))
	})

	It("podman network works across user ns", func() {
		netName := createNetworkName("")
		create := podmanTest.Podman([]string{"network", "create", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		name := "nc-server"
		run := podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "-d", "--name", name, "--net", netName, ALPINE, "nc", "-l", "-p", "9480"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		// NOTE: we force the k8s-file log driver to make sure the
		// tests are passing inside a container.
		// "sleep" needed to give aardvark-dns time to come up; #16272
		run = podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "--rm", "--net", netName, "--uidmap", "0:1:4096", ALPINE, "sh", "-c", fmt.Sprintf("sleep 2;echo podman | nc -w 1 %s.dns.podman 9480", name)})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		log := podmanTest.Podman([]string{"logs", name})
		log.WaitWithDefaultTimeout()
		Expect(log).Should(ExitCleanly())
		Expect(log.OutputToString()).To(Equal("podman"))
	})

	It("podman run with new:pod and static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.40.128"
		podname := "testpod"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.40.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		run := podmanTest.Podman([]string{"run", "--rm", "--pod", "new:" + podname, "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))

		podrm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podname})
		podrm.WaitWithDefaultTimeout()
		Expect(podrm).Should(ExitCleanly())
	})

	It("podman run with --net=host and --hostname sets correct hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=host", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with --net=none sets hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with --net=none adds hostname to /etc/hosts", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with pod does not add extra 127 entry to /etc/hosts", func() {
		pod := "testpod"
		hostname := "test-hostname"
		run := podmanTest.Podman([]string{"pod", "create", "--hostname", hostname, "--name", pod})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		run = podmanTest.Podman([]string{"run", "--pod", pod, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).ToNot(ContainSubstring("127.0.0.1 %s", hostname))
	})

	pingTest := func(netns string) {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", netns, "--cap-add", "net_raw", "--hostname", hostname, ALPINE, "ping", "-c", "1", hostname})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		run = podmanTest.Podman([]string{"run", netns, "--cap-add", "net_raw", "--hostname", hostname, "--name", "test", ALPINE, "ping", "-c", "1", "test"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
	}

	It("podman attempt to ping container name and hostname --net=none", func() {
		pingTest("--net=none")
	})

	It("podman attempt to ping container name and hostname --net=private", func() {
		pingTest("--net=private")
	})

	It("podman run check dns", func() {
		SkipIfCNI(podmanTest)
		pod := "testpod"
		session := podmanTest.Podman([]string{"pod", "create", "--name", pod})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		net := createNetworkName("IntTest")
		session = podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		pod2 := "testpod2"
		hostname := "hostn1"
		session = podmanTest.Podman([]string{"pod", "create", "--network", net, "--name", pod2, "--hostname", hostname})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con1", "--network", net, CITEST_IMAGE, "nslookup", "con1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con2", "--pod", pod, "--network", net, CITEST_IMAGE, "nslookup", "con2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con3", "--pod", pod2, CITEST_IMAGE, "nslookup", "con1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(ContainSubstring("server can't find con1.dns.podman: NXDOMAIN"))

		session = podmanTest.Podman([]string{"run", "--name", "con4", "--network", net, CITEST_IMAGE, "nslookup", pod2 + ".dns.podman"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, CITEST_IMAGE, "nslookup", hostname})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman network adds dns search domain with dns", func() {
		net := createNetworkName("dnsname")
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("search dns.podman"))
	})

	It("Rootless podman run with --net=bridge works and connects to default network", func() {
		// This is harmless when run as root, so we'll just let it run.
		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--net=bridge", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Networks).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Networks).To(HaveKey("podman"))
	})

	// see https://github.com/containers/podman/issues/12972
	It("podman run check network-alias works on networks without dns", func() {
		net := "dns" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--disable-dns", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, "--network-alias", "abcdef", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run with ipam none driver", func() {
		net := "ipam" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--ipam-driver=none", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, ALPINE, "ip", "addr", "show", "eth0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4), "output should only show link local address")
	})

	It("podman run with macvlan network", func() {
		net := "mv-" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "--subnet", "10.10.0.0/24", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		// use options and search to make sure we get the same resolv.conf everywhere
		run := podmanTest.Podman([]string{"run", "--network", net, "--dns", "127.0.0.128",
			"--dns-option", "ndots:1", "--dns-search", ".", ALPINE, "cat", "/etc/resolv.conf"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(string(run.Out.Contents())).To(Equal(`nameserver 127.0.0.128
options ndots:1
`))
	})
})
