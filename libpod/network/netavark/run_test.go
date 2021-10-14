// +build linux

package netavark_test

// The tests have to be run as root.
// For each test there will be two network namespaces created,
// netNSTest and netNSContainer. Each test must be run inside
// netNSTest to prevent leakage in the host netns, therefore
// it should use the following structure:
// It("test name", func() {
//   runTest(func() {
//     // add test logic here
//   })
// })

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/netns"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
)

var _ = Describe("run netavark", func() {
	var (
		libpodNet      types.ContainerNetwork
		confDir        string
		logBuffer      bytes.Buffer
		netNSTest      ns.NetNS
		netNSContainer ns.NetNS
	)

	// runTest is a helper function to run a test. It ensures that each test
	// is run in its own netns. It also creates a mountns to mount a tmpfs to /var/lib/cni.
	runTest := func(run func()) {
		netNSTest.Do(func(_ ns.NetNS) error {
			defer GinkgoRecover()
			// we have to setup the loopback adapter in this netns to use port forwarding
			link, err := netlink.LinkByName("lo")
			Expect(err).To(BeNil(), "Failed to get loopback adapter")
			err = netlink.LinkSetUp(link)
			Expect(err).To(BeNil(), "Failed to set loopback adapter up")
			run()
			return nil
		})
	}

	BeforeEach(func() {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.SetFormatter(&logrus.TextFormatter{DisableQuote: true})
		// The tests need root privileges.
		// Technically we could work around that by using user namespaces and
		// the rootless cni code but this is to much work to get it right for a unit test.
		if rootless.IsRootless() {
			Skip("this test needs to be run as root")
		}

		var err error
		confDir, err = ioutil.TempDir("", "podman_netavark_test")
		if err != nil {
			Fail("Failed to create tmpdir")
		}
		logBuffer = bytes.Buffer{}
		logrus.SetOutput(&logBuffer)

		netNSTest, err = netns.NewNS()
		if err != nil {
			Fail("Failed to create netns")
		}

		netNSContainer, err = netns.NewNS()
		if err != nil {
			Fail("Failed to create netns")
		}
	})

	JustBeforeEach(func() {
		var err error
		libpodNet, err = getNetworkInterface(confDir, false)
		if err != nil {
			Fail("Failed to create NewCNINetworkInterface")
		}
	})

	AfterEach(func() {
		logrus.SetFormatter(&logrus.TextFormatter{})
		logrus.SetLevel(logrus.InfoLevel)
		os.RemoveAll(confDir)

		netns.UnmountNS(netNSTest)
		netNSTest.Close()

		netns.UnmountNS(netNSContainer)
		netNSContainer.Close()

		fmt.Println(logBuffer.String())
	})

	It("test basic setup", func() {
		runTest(func() {
			defNet := types.DefaultNetworkName
			intName := "eth0"
			opts := types.SetupOptions{
				NetworkOptions: types.NetworkOptions{
					ContainerID:   "someID",
					ContainerName: "someName",
					Networks: map[string]types.PerNetworkOptions{
						defNet: {
							InterfaceName: intName,
						},
					},
				},
			}
			res, err := libpodNet.Setup(netNSContainer.Path(), opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(HaveLen(1))
			Expect(res).To(HaveKey(defNet))
			Expect(res[defNet].Interfaces).To(HaveKey(intName))
			Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
			ip := res[defNet].Interfaces[intName].Networks[0].Subnet.IP
			Expect(ip.String()).To(ContainSubstring("10.88.0."))
			gw := res[defNet].Interfaces[intName].Networks[0].Gateway
			Expect(gw.String()).To(Equal("10.88.0.1"))
			macAddress := res[defNet].Interfaces[intName].MacAddress
			Expect(macAddress).To(HaveLen(6))
			// default network has no dns
			Expect(res[defNet].DNSServerIPs).To(BeEmpty())
			Expect(res[defNet].DNSSearchDomains).To(BeEmpty())

			// check in the container namespace if the settings are applied
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				defer GinkgoRecover()
				i, err := net.InterfaceByName(intName)
				Expect(err).To(BeNil())
				Expect(i.Name).To(Equal(intName))
				Expect(i.HardwareAddr).To(Equal(macAddress))
				addrs, err := i.Addrs()
				Expect(err).To(BeNil())
				subnet := &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(16, 32),
				}
				Expect(addrs).To(ContainElements(subnet))

				// check loopback adapter
				i, err = net.InterfaceByName("lo")
				Expect(err).To(BeNil())
				Expect(i.Name).To(Equal("lo"))
				Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
				Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")
				return nil
			})
			Expect(err).To(BeNil())

			// default bridge name
			bridgeName := "podman0"
			// check settings on the host side
			i, err := net.InterfaceByName(bridgeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(i.Name).To(Equal(bridgeName))
			addrs, err := i.Addrs()
			Expect(err).ToNot(HaveOccurred())
			// test that the gateway ip is assigned to the interface
			subnet := &net.IPNet{
				IP:   gw,
				Mask: net.CIDRMask(16, 32),
			}
			Expect(addrs).To(ContainElements(subnet))

			wg := &sync.WaitGroup{}
			expected := stringid.GenerateNonCryptoID()
			// now check ip connectivity
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				runNetListener(wg, "tcp", "0.0.0.0", 5000, expected)
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			conn, err := net.Dial("tcp", ip.String()+":5000")
			Expect(err).To(BeNil())
			_, err = conn.Write([]byte(expected))
			Expect(err).To(BeNil())
			conn.Close()

			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(opts))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	for _, proto := range []string{"tcp", "udp"} {
		// copy proto to extra var to keep correct references in the goroutines
		protocol := proto
		It("run with exposed ports protocol "+protocol, func() {
			runTest(func() {
				testdata := stringid.GenerateNonCryptoID()
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateNonCryptoID(),
						PortMappings: []types.PortMapping{{
							Protocol:      protocol,
							HostIP:        "127.0.0.1",
							HostPort:      5000,
							ContainerPort: 5000,
						}},
						Networks: map[string]types.PerNetworkOptions{
							defNet: {InterfaceName: intName},
						},
					},
				}
				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(defNet))
				Expect(res[defNet].Interfaces).To(HaveKey(intName))
				Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
				Expect(res[defNet].Interfaces[intName].Networks[0].Subnet.IP.String()).To(ContainSubstring("10.88.0."))
				Expect(res[defNet].Interfaces[intName].MacAddress).To(HaveLen(6))
				// default network has no dns
				Expect(res[defNet].DNSServerIPs).To(BeEmpty())
				Expect(res[defNet].DNSSearchDomains).To(BeEmpty())
				var wg sync.WaitGroup
				wg.Add(1)
				// start a listener in the container ns
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					runNetListener(&wg, protocol, "0.0.0.0", 5000, testdata)
					return nil
				})
				Expect(err).To(BeNil())

				conn, err := net.Dial(protocol, "127.0.0.1:5000")
				Expect(err).To(BeNil())
				_, err = conn.Write([]byte(testdata))
				Expect(err).To(BeNil())
				conn.Close()

				// wait for the listener to finish
				wg.Wait()

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
			})
		})

		It("run with range ports protocol "+protocol, func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateNonCryptoID(),
						PortMappings: []types.PortMapping{{
							Protocol:      protocol,
							HostIP:        "127.0.0.1",
							HostPort:      5001,
							ContainerPort: 5000,
							Range:         3,
						}},
						Networks: map[string]types.PerNetworkOptions{
							defNet: {InterfaceName: intName},
						},
					},
				}
				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(defNet))
				Expect(res[defNet].Interfaces).To(HaveKey(intName))
				Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
				containerIP := res[defNet].Interfaces[intName].Networks[0].Subnet.IP.String()
				Expect(containerIP).To(ContainSubstring("10.88.0."))
				Expect(res[defNet].Interfaces[intName].MacAddress).To(HaveLen(6))
				// default network has no dns
				Expect(res[defNet].DNSServerIPs).To(BeEmpty())
				Expect(res[defNet].DNSSearchDomains).To(BeEmpty())

				// loop over all ports
				for p := 5001; p < 5004; p++ {
					port := p
					var wg sync.WaitGroup
					wg.Add(1)
					testdata := stringid.GenerateNonCryptoID()
					// start a listener in the container ns
					err = netNSContainer.Do(func(_ ns.NetNS) error {
						defer GinkgoRecover()
						runNetListener(&wg, protocol, containerIP, port-1, testdata)
						return nil
					})
					Expect(err).To(BeNil())

					conn, err := net.Dial(protocol, net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
					Expect(err).To(BeNil())
					_, err = conn.Write([]byte(testdata))
					Expect(err).To(BeNil())
					conn.Close()

					// wait for the listener to finish
					wg.Wait()
				}

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
			})
		})
	}

})

func runNetListener(wg *sync.WaitGroup, protocol, ip string, port int, expectedData string) {
	switch protocol {
	case "tcp":
		ln, err := net.Listen(protocol, net.JoinHostPort(ip, strconv.Itoa(port)))
		Expect(err).To(BeNil())
		// make sure to read in a separate goroutine to not block
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			defer ln.Close()
			conn, err := ln.Accept()
			Expect(err).To(BeNil())
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(1 * time.Second))
			data, err := ioutil.ReadAll(conn)
			Expect(err).To(BeNil())
			Expect(string(data)).To(Equal(expectedData))
		}()
	case "udp":
		conn, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		})
		Expect(err).To(BeNil())
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			defer conn.Close()
			data := make([]byte, len(expectedData))
			i, err := conn.Read(data)
			Expect(err).To(BeNil())
			Expect(i).To(Equal(len(expectedData)))
			Expect(string(data)).To(Equal(expectedData))
		}()
	default:
		Fail("unsupported protocol")
	}
}
