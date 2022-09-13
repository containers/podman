package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/uber/jaeger-client-go/utils"
)

var _ = Describe("Podman run networking", func() {
	var (
		tempdir     string
		err         error
		podmanTest  *PodmanTestIntegration
		hostname, _ = os.Hostname()
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

	It("podman run network connection with default bridge", func() {
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE, "wget", "www.podman.io"})
		session.Wait(90)
		Expect(session).Should(Exit(0))
	})

	It("podman run network connection with host", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--network", "host", ALPINE, "wget", "www.podman.io"})
		session.Wait(90)
		Expect(session).Should(Exit(0))
	})

	It("podman run network connection with default", func() {
		session := podmanTest.Podman([]string{"run", "--network", "default", ALPINE, "wget", "www.podman.io"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run network connection with none", func() {
		session := podmanTest.Podman([]string{"run", "--network", "none", ALPINE, "wget", "www.podman.io"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("wget: bad address 'www.podman.io'"))
	})

	It("podman run network connection with private", func() {
		session := podmanTest.Podman([]string{"run", "--network", "private", ALPINE, "wget", "www.podman.io"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run network connection with loopback", func() {
		session := podmanTest.Podman([]string{"run", "--network", "host", ALPINE, "wget", "www.podman.io"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run network expose port 222", func() {
		SkipIfRootless("iptables is not supported for rootless users")
		session := podmanTest.Podman([]string{"run", "-dt", "--expose", "222-223", "-P", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		results := SystemExec("iptables", []string{"-t", "nat", "-L"})
		Expect(results).Should(Exit(0))
		Expect(results.OutputToString()).To(ContainSubstring("222"))
		Expect(results.OutputToString()).To(ContainSubstring("223"))
	})

	It("podman run -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 80-82 -p 8090:8090", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-82", "-p", "8090:8090", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(4))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["81/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["82/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0].HostPort).To(Not(Equal("82")))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8090/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0].HostPort).To(Equal("8090"))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 80-81 -p 8180-8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-81", "-p", "8180-8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(4))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["81/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8180/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0].HostPort).To(Not(Equal("8180")))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8181/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 80 -p 8280-8282:8280-8282", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "-p", "8280-8282:8280-8282", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(4))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8280/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0].HostPort).To(Equal("8280"))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8281/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0].HostPort).To(Equal("8281"))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0].HostIP).To(Equal(""))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8282/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0].HostPort).To(Equal("8282"))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 8380:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8380:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Equal("8380"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 8480:80/TCP", func() {
		name := "testctr"
		// "TCP" in upper characters
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8480:80/TCP", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		// "tcp" in lower characters
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Equal("8480"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/udp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 127.0.0.1:8580:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8580:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Equal("8580"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal("127.0.0.1"))
	})

	It("podman run -p 127.0.0.1:8680:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8680:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/udp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Equal("8680"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostIP).To(Equal("127.0.0.1"))
	})

	It("podman run -p [::1]:8780:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8780:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/udp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Equal("8780"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostIP).To(Equal("::1"))
	})

	It("podman run -p [::1]:8880:80/tcp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8880:80/tcp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Equal("8880"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal("::1"))
	})

	It("podman run --expose 80 -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--expose", "80", "-P", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run --expose 80/udp -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--expose", "80/udp", "-P", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/udp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostIP).To(Equal(""))
	})

	It("podman run --expose 80 -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--expose", "80", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p 127.0.0.1::8980/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1::8980/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8980/udp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0].HostPort).To(Not(Equal("8980")))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0].HostIP).To(Equal("127.0.0.1"))
	})

	It("podman run -p :8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", ":8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8181/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run -p xxx:8080 -p yyy:8080", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "4444:8080", "-p", "5555:8080", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["8080/tcp"])).To(Equal(2))

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
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Ports["80/tcp"])).To(Equal(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Equal("9280"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostIP).To(Equal(""))
	})

	It("podman run network expose host port 80 to container port 8000", func() {
		SkipIfRootless("iptables is not supported for rootless users")
		session := podmanTest.Podman([]string{"run", "-dt", "-p", "80:8000", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		results := SystemExec("iptables", []string{"-t", "nat", "-L"})
		Expect(results).Should(Exit(0))
		Expect(results.OutputToString()).To(ContainSubstring("8000"))

		ncBusy := SystemExec("nc", []string{"-l", "-p", "80"})
		Expect(ncBusy).To(ExitWithError())
	})

	It("podman run network expose host port 18081 to container port 8000 using rootlesskit port handler", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:port_handler=rootlesskit", "-dt", "-p", "18081:8000", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session).Should(Exit(0))

		ncBusy := SystemExec("nc", []string{"-l", "-p", "18081"})
		Expect(ncBusy).To(ExitWithError())
	})

	It("podman run slirp4netns verify net.ipv6.conf.default.accept_dad=0", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:enable_ipv6=true", ALPINE, "ip", "addr"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		// check the ipv6 setup id done without delay (https://github.com/containers/podman/issues/11062)
		Expect(session.OutputToString()).To(ContainSubstring("inet6 fd00::"))

		const ipv6ConfDefaultAcceptDadSysctl = "/proc/sys/net/ipv6/conf/all/accept_dad"

		cat := SystemExec("cat", []string{ipv6ConfDefaultAcceptDadSysctl})
		cat.Wait(30)
		Expect(cat).Should(Exit(0))
		sysctlValue := cat.OutputToString()

		session = podmanTest.Podman([]string{"run", "--network", "slirp4netns:enable_ipv6=true", ALPINE, "cat", ipv6ConfDefaultAcceptDadSysctl})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(sysctlValue))
	})

	It("podman run network expose host port 18082 to container port 8000 using slirp4netns port handler", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:port_handler=slirp4netns", "-dt", "-p", "18082:8000", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		ncBusy := SystemExec("nc", []string{"-l", "-p", "18082"})
		Expect(ncBusy).To(ExitWithError())
	})

	It("podman run network expose host port 8080 to container port 8000 using invalid port handler", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:port_handler=invalid", "-dt", "-p", "8080:8000", ALPINE, "/bin/sh"})
		session.Wait(30)
		Expect(session).To(ExitWithError())
	})

	It("podman run slirp4netns network with host loopback", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:allow_host_loopback=true", ALPINE, "ping", "-c1", "10.0.2.2"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
	})

	It("podman run slirp4netns network with mtu", func() {
		session := podmanTest.Podman([]string{"run", "--network", "slirp4netns:mtu=9000", ALPINE, "ip", "addr"})
		session.Wait(30)
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("mtu 9000"))
	})

	It("podman run slirp4netns network with different cidr", func() {
		slirp4netnsHelp := SystemExec("slirp4netns", []string{"--help"})
		Expect(slirp4netnsHelp).Should(Exit(0))

		networkConfiguration := "slirp4netns:cidr=192.168.0.0/24,allow_host_loopback=true"
		session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, ALPINE, "ping", "-c1", "192.168.0.2"})
		session.Wait(30)

		if strings.Contains(slirp4netnsHelp.OutputToString(), "cidr") {
			Expect(session).Should(Exit(0))
		} else {
			Expect(session).To(ExitWithError())
			Expect(session.ErrorToString()).To(ContainSubstring("cidr not supported"))
		}
	})

	It("podman run network bind to 127.0.0.1", func() {
		slirp4netnsHelp := SystemExec("slirp4netns", []string{"--help"})
		Expect(slirp4netnsHelp).Should(Exit(0))
		networkConfiguration := "slirp4netns:outbound_addr=127.0.0.1,allow_host_loopback=true"

		if strings.Contains(slirp4netnsHelp.OutputToString(), "outbound-addr") {
			ncListener := StartSystemExec("nc", []string{"-v", "-n", "-l", "-p", "8083"})
			session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, "-dt", ALPINE, "nc", "-w", "2", "10.0.2.2", "8083"})
			session.Wait(30)
			ncListener.Wait(30)

			Expect(session).Should(Exit(0))
			Expect(ncListener).Should(Exit(0))
			Expect(ncListener.ErrorToString()).To(ContainSubstring("127.0.0.1"))
		} else {
			session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, "-dt", ALPINE, "nc", "-w", "2", "10.0.2.2", "8083"})
			session.Wait(30)
			Expect(session).To(ExitWithError())
			Expect(session.ErrorToString()).To(ContainSubstring("outbound_addr not supported"))
		}
	})

	It("podman run network bind to HostIP", func() {
		ip, err := utils.HostIP()
		Expect(err).To(BeNil())

		slirp4netnsHelp := SystemExec("slirp4netns", []string{"--help"})
		Expect(slirp4netnsHelp).Should(Exit(0))
		networkConfiguration := fmt.Sprintf("slirp4netns:outbound_addr=%s,allow_host_loopback=true", ip.String())

		if strings.Contains(slirp4netnsHelp.OutputToString(), "outbound-addr") {
			ncListener := StartSystemExec("nc", []string{"-v", "-n", "-l", "-p", "8084"})
			session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, "-dt", ALPINE, "nc", "-w", "2", "10.0.2.2", "8084"})
			session.Wait(30)
			ncListener.Wait(30)

			Expect(session).Should(Exit(0))
			Expect(ncListener).Should(Exit(0))
			Expect(ncListener.ErrorToString()).To(ContainSubstring(ip.String()))
		} else {
			session := podmanTest.Podman([]string{"run", "--network", networkConfiguration, "-dt", ALPINE, "nc", "-w", "2", "10.0.2.2", "8084"})
			session.Wait(30)
			Expect(session).To(ExitWithError())
			Expect(session.ErrorToString()).To(ContainSubstring("outbound_addr not supported"))
		}
	})

	It("podman run network expose ports in image metadata", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "-t", "-P", nginx})
		session.Wait(90)
		Expect(session).Should(Exit(0))
		results := podmanTest.Podman([]string{"inspect", "test"})
		results.Wait(30)
		Expect(results).Should(Exit(0))
		Expect(results.OutputToString()).To(ContainSubstring(`"80/tcp":`))
	})

	It("podman run network expose duplicate host port results in error", func() {
		SkipIfRootless("FIXME we should be able to run this test in rootless mode with different ports")

		session := podmanTest.Podman([]string{"run", "--name", "test", "-dt", "-p", "80", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		containerConfig := inspect.InspectContainerToJSON()
		Expect(containerConfig[0].NetworkSettings.Ports).To(Not(BeNil()))
		Expect(containerConfig[0].NetworkSettings.Ports["80/tcp"]).To(Not(BeNil()))
		Expect(containerConfig[0].NetworkSettings.Ports["80/tcp"][0].HostPort).ToNot(Equal(80))
	})

	It("podman run hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString(hostname)
		Expect(match).Should(BeFalse())
	})

	It("podman run --net host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString(hostname)
		Expect(match).Should(BeTrue())
	})
	It("podman run --net host --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString(hostname)
		Expect(match).Should(BeTrue())
	})
	It("podman run --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString(hostname)
		Expect(match).Should(BeTrue())
	})

	It("podman run --net host --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString("foobar")
		Expect(match).Should(BeTrue())
	})

	It("podman run --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		match, _ := session.GrepString("foobar")
		Expect(match).Should(BeTrue())
	})

	It("podman run --net container: and --uts container:", func() {
		ctrName := "ctrToJoin"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--uts=container:" + ctrName, ALPINE, "true"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))
	})

	It("podman run --net container: copies hosts and resolv", func() {
		ctrName := "ctr1"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		// Exec in and modify /etc/resolv.conf and /etc/hosts
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo nameserver 192.0.2.1 > /etc/resolv.conf"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(Exit(0))

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo 192.0.2.2 test1 > /etc/hosts"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(Exit(0))

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--name", ctrName2, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		exec3 := podmanTest.Podman([]string{"exec", "-i", ctrName2, "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(Exit(0))
		Expect(exec3.OutputToString()).To(ContainSubstring("nameserver 192.0.2.1"))

		exec4 := podmanTest.Podman([]string{"exec", "-i", ctrName2, "cat", "/etc/hosts"})
		exec4.WaitWithDefaultTimeout()
		Expect(exec4).Should(Exit(0))
		Expect(exec4.OutputToString()).To(ContainSubstring("192.0.2.2 test1"))
	})

	It("podman run /etc/hosts contains --hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run --uidmap /etc/hosts contains --hostname", func() {
		SkipIfRootless("uidmap population of cninetworks not supported for rootless users")
		session := podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", "-v", "/etc/hosts:/etc/hosts", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("podman run network in user created network namespace", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX := SystemExec("ip", []string{"netns", "add", "xxx"})
		Expect(addXXX).Should(Exit(0))
		defer func() {
			delXXX := SystemExec("ip", []string{"netns", "delete", "xxx"})
			Expect(delXXX).Should(Exit(0))
		}()

		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxx", ALPINE, "wget", "www.podman.io"})
		session.Wait(90)
		Expect(session).Should(Exit(0))
	})

	It("podman run n user created network namespace with resolv.conf", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX2 := SystemExec("ip", []string{"netns", "add", "xxx2"})
		Expect(addXXX2).Should(Exit(0))
		defer func() {
			delXXX2 := SystemExec("ip", []string{"netns", "delete", "xxx2"})
			Expect(delXXX2).Should(Exit(0))
		}()

		mdXXX2 := SystemExec("mkdir", []string{"-p", "/etc/netns/xxx2"})
		Expect(mdXXX2).Should(Exit(0))
		defer os.RemoveAll("/etc/netns/xxx2")

		nsXXX2 := SystemExec("bash", []string{"-c", "echo nameserver 11.11.11.11 > /etc/netns/xxx2/resolv.conf"})
		Expect(nsXXX2).Should(Exit(0))

		session := podmanTest.Podman([]string{"run", "--net", "ns:/run/netns/xxx2", ALPINE, "cat", "/etc/resolv.conf"})
		session.Wait(90)
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("11.11.11.11"))
	})

	It("podman run network in bogus user created network namespace", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxy", ALPINE, "wget", "www.podman.io"})
		session.Wait(90)
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("stat /run/netns/xxy: no such file or directory"))
	})

	It("podman run in custom CNI network with --static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.30.128"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.30.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))
		defer podmanTest.removeCNINetwork(netName)

		run := podmanTest.Podman([]string{"run", "-t", "-i", "--rm", "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))
	})

	It("podman cni network works across user ns", func() {
		netName := stringid.GenerateRandomID()
		create := podmanTest.Podman([]string{"network", "create", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))
		defer podmanTest.removeCNINetwork(netName)

		name := "nc-server"
		run := podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "-d", "--name", name, "--net", netName, ALPINE, "nc", "-l", "-p", "9480"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		// NOTE: we force the k8s-file log driver to make sure the
		// tests are passing inside a container.
		run = podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "--rm", "--net", netName, "--uidmap", "0:1:4096", ALPINE, "sh", "-c", fmt.Sprintf("echo podman | nc -w 1 %s.dns.podman 9480", name)})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		log := podmanTest.Podman([]string{"logs", name})
		log.WaitWithDefaultTimeout()
		Expect(log).Should(Exit(0))
		Expect(log.OutputToString()).To(Equal("podman"))
	})

	It("podman run with new:pod and static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.40.128"
		podname := "testpod"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.40.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))
		defer podmanTest.removeCNINetwork(netName)

		run := podmanTest.Podman([]string{"run", "-t", "-i", "--rm", "--pod", "new:" + podname, "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))

		podrm := podmanTest.Podman([]string{"pod", "rm", "-f", podname})
		podrm.WaitWithDefaultTimeout()
		Expect(podrm).Should(Exit(0))
	})

	It("podman run with --net=host and --hostname sets correct hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=host", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(strings.Contains(run.OutputToString(), hostname)).To(BeTrue())
	})

	It("podman run with --net=none sets hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(strings.Contains(run.OutputToString(), hostname)).To(BeTrue())
	})

	It("podman run with --net=none adds hostname to /etc/hosts", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(strings.Contains(run.OutputToString(), hostname)).To(BeTrue())
	})

	It("podman run with pod does not add extra 127 entry to /etc/hosts", func() {
		pod := "testpod"
		hostname := "test-hostname"
		run := podmanTest.Podman([]string{"pod", "create", "--hostname", hostname, "--name", pod})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		run = podmanTest.Podman([]string{"run", "--pod", pod, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).ToNot(ContainSubstring("127.0.0.1 %s", hostname))
	})

	ping_test := func(netns string) {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", netns, "--hostname", hostname, ALPINE, "ping", "-c", "1", hostname})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		run = podmanTest.Podman([]string{"run", netns, "--hostname", hostname, "--name", "test", ALPINE, "ping", "-c", "1", "test"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
	}

	It("podman attempt to ping container name and hostname --net=none", func() {
		ping_test("--net=none")
	})

	It("podman attempt to ping container name and hostname --net=private", func() {
		ping_test("--net=private")
	})

	It("podman run check dnsname plugin", func() {
		pod := "testpod"
		session := podmanTest.Podman([]string{"pod", "create", "--name", pod})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		net := "IntTest" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		pod2 := "testpod2"
		session = podmanTest.Podman([]string{"pod", "create", "--network", net, "--name", pod2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", "con1", "--network", net, ALPINE, "nslookup", "con1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", "con2", "--pod", pod, "--network", net, ALPINE, "nslookup", "con2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--name", "con3", "--pod", pod2, ALPINE, "nslookup", "con1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("can't resolve 'con1'"))

		session = podmanTest.Podman([]string{"run", "--name", "con4", "--network", net, ALPINE, "nslookup", pod2 + ".dns.podman"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman run check dnsname adds dns search domain", func() {
		net := "dnsname" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeCNINetwork(net)
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--network", net, ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("search dns.podman"))
	})

	It("Rootless podman run with --net=bridge works and connects to default network", func() {
		// This is harmless when run as root, so we'll just let it run.
		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--net=bridge", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(Exit(0))

		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(len(inspectOut)).To(Equal(1))
		Expect(len(inspectOut[0].NetworkSettings.Networks)).To(Equal(1))
		_, ok := inspectOut[0].NetworkSettings.Networks["podman"]
		Expect(ok).To(BeTrue())
	})
})
