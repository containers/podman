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
	"github.com/containers/podman/v3/libpod/network/util"
	"github.com/containers/podman/v3/pkg/netns"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
)

var _ = Describe("run netavark", func() {
	var (
		libpodNet      types.ContainerNetwork
		confDir        string
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
		if _, ok := os.LookupEnv("NETAVARK_BINARY"); !ok {
			Skip("NETAVARK_BINARY not set skip run tests")
		}

		// set the logrus settings
		logrus.SetLevel(logrus.TraceLevel)
		// disable extra quotes so we can easily copy the netavark command
		logrus.SetFormatter(&logrus.TextFormatter{DisableQuote: true})
		logrus.SetOutput(os.Stderr)
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
			util.NormalizeIP(&gw)
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
				Expect(i.HardwareAddr).To(Equal(net.HardwareAddr(macAddress)))
				addrs, err := i.Addrs()
				Expect(err).To(BeNil())
				subnet := &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(16, 32),
				}
				Expect(addrs).To(ContainElements(EqualSubnet(subnet)))

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
			Expect(addrs).To(ContainElements(EqualSubnet(subnet)))

			wg := &sync.WaitGroup{}
			expected := stringid.GenerateNonCryptoID()
			// now check ip connectivity
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				wg.Add(1)
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
			wg.Wait()
		})
	})

	It("setup two containers", func() {
		runTest(func() {
			defNet := types.DefaultNetworkName
			intName := "eth0"
			setupOpts1 := types.SetupOptions{
				NetworkOptions: types.NetworkOptions{
					ContainerID: stringid.GenerateNonCryptoID(),
					Networks: map[string]types.PerNetworkOptions{
						defNet: {InterfaceName: intName},
					},
				},
			}
			res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts1)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(HaveLen(1))
			Expect(res).To(HaveKey(defNet))
			Expect(res[defNet].Interfaces).To(HaveKey(intName))
			Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
			ip1 := res[defNet].Interfaces[intName].Networks[0].Subnet.IP
			Expect(ip1.String()).To(ContainSubstring("10.88.0."))
			Expect(res[defNet].Interfaces[intName].MacAddress).To(HaveLen(6))

			setupOpts2 := types.SetupOptions{
				NetworkOptions: types.NetworkOptions{
					ContainerID: stringid.GenerateNonCryptoID(),
					Networks: map[string]types.PerNetworkOptions{
						defNet: {InterfaceName: intName},
					},
				},
			}

			netNSContainer2, err := netns.NewNS()
			Expect(err).ToNot(HaveOccurred())
			defer netns.UnmountNS(netNSContainer2)
			defer netNSContainer2.Close()

			res, err = libpodNet.Setup(netNSContainer2.Path(), setupOpts2)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(HaveLen(1))
			Expect(res).To(HaveKey(defNet))
			Expect(res[defNet].Interfaces).To(HaveKey(intName))
			Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
			ip2 := res[defNet].Interfaces[intName].Networks[0].Subnet.IP
			Expect(ip2.String()).To(ContainSubstring("10.88.0."))
			Expect(res[defNet].Interfaces[intName].MacAddress).To(HaveLen(6))
			Expect(ip1.Equal(ip2)).To(BeFalse(), "IP1 %s should not be equal to IP2 %s", ip1.String(), ip2.String())

			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts1))
			Expect(err).ToNot(HaveOccurred())
			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts2))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("setup dualstack network", func() {
		runTest(func() {
			s1, _ := types.ParseCIDR("10.0.0.1/24")
			s2, _ := types.ParseCIDR("fd10:88:a::/64")
			network, err := libpodNet.NetworkCreate(types.Network{
				Subnets: []types.Subnet{
					{Subnet: s1}, {Subnet: s2},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			netName := network.Name
			intName := "eth0"

			setupOpts := types.SetupOptions{
				NetworkOptions: types.NetworkOptions{
					ContainerID: stringid.GenerateNonCryptoID(),
					Networks: map[string]types.PerNetworkOptions{
						netName: {InterfaceName: intName},
					},
				},
			}
			res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(HaveLen(1))
			Expect(res).To(HaveKey(netName))
			Expect(res[netName].Interfaces).To(HaveKey(intName))
			Expect(res[netName].Interfaces[intName].Networks).To(HaveLen(2))
			ip1 := res[netName].Interfaces[intName].Networks[0].Subnet.IP
			Expect(ip1.String()).To(ContainSubstring("10.0.0."))
			gw1 := res[netName].Interfaces[intName].Networks[0].Gateway
			Expect(gw1.String()).To(Equal("10.0.0.1"))
			ip2 := res[netName].Interfaces[intName].Networks[1].Subnet.IP
			Expect(ip2.String()).To(ContainSubstring("fd10:88:a::"))
			gw2 := res[netName].Interfaces[intName].Networks[0].Gateway
			Expect(gw2.String()).To(Equal("fd10:88:a::1"))
			Expect(res[netName].Interfaces[intName].MacAddress).To(HaveLen(6))

			// check in the container namespace if the settings are applied
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				defer GinkgoRecover()
				i, err := net.InterfaceByName(intName)
				Expect(err).ToNot(HaveOccurred())
				Expect(i.Name).To(Equal(intName))
				addrs, err := i.Addrs()
				Expect(err).ToNot(HaveOccurred())
				subnet1 := s1.IPNet
				subnet1.IP = ip1
				subnet2 := s2.IPNet
				subnet2.IP = ip2
				Expect(addrs).To(ContainElements(EqualSubnet(&subnet1), EqualSubnet(&subnet2)))

				// check loopback adapter
				i, err = net.InterfaceByName("lo")
				Expect(err).ToNot(HaveOccurred())
				Expect(i.Name).To(Equal("lo"))
				Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
				Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			bridgeName := network.NetworkInterface
			// check settings on the host side
			i, err := net.InterfaceByName(bridgeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(i.Name).To(Equal(bridgeName))
			addrs, err := i.Addrs()
			Expect(err).ToNot(HaveOccurred())
			// test that the gateway ip is assigned to the interface
			subnet1 := s1.IPNet
			subnet1.IP = gw1
			subnet2 := s2.IPNet
			subnet2.IP = gw2
			Expect(addrs).To(ContainElements(EqualSubnet(&subnet1), EqualSubnet(&subnet2)))

			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("setup two networks", func() {
		runTest(func() {
			s1, _ := types.ParseCIDR("10.0.0.1/24")
			network1, err := libpodNet.NetworkCreate(types.Network{
				Subnets: []types.Subnet{
					{Subnet: s1},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			netName1 := network1.Name
			intName1 := "eth0"

			s2, _ := types.ParseCIDR("10.1.0.0/24")
			network2, err := libpodNet.NetworkCreate(types.Network{
				Subnets: []types.Subnet{
					{Subnet: s2},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			netName2 := network2.Name
			intName2 := "eth1"

			setupOpts := types.SetupOptions{
				NetworkOptions: types.NetworkOptions{
					ContainerID: stringid.GenerateNonCryptoID(),
					Networks: map[string]types.PerNetworkOptions{
						netName1: {InterfaceName: intName1},
						netName2: {InterfaceName: intName2},
					},
				},
			}
			res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(HaveLen(2))
			Expect(res).To(HaveKey(netName1))
			Expect(res).To(HaveKey(netName2))
			Expect(res[netName1].Interfaces).To(HaveKey(intName1))
			Expect(res[netName2].Interfaces).To(HaveKey(intName2))
			Expect(res[netName1].Interfaces[intName1].Networks).To(HaveLen(1))
			ip1 := res[netName1].Interfaces[intName1].Networks[0].Subnet.IP
			Expect(ip1.String()).To(ContainSubstring("10.0.0."))
			gw1 := res[netName1].Interfaces[intName1].Networks[0].Gateway
			Expect(gw1.String()).To(Equal("10.0.0.1"))
			ip2 := res[netName2].Interfaces[intName2].Networks[0].Subnet.IP
			Expect(ip2.String()).To(ContainSubstring("10.1.0."))
			gw2 := res[netName2].Interfaces[intName2].Networks[0].Gateway
			Expect(gw2.String()).To(Equal("10.1.0.1"))
			mac1 := res[netName1].Interfaces[intName1].MacAddress
			Expect(mac1).To(HaveLen(6))
			mac2 := res[netName2].Interfaces[intName2].MacAddress
			Expect(mac2).To(HaveLen(6))

			// check in the container namespace if the settings are applied
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				defer GinkgoRecover()
				i, err := net.InterfaceByName(intName1)
				Expect(err).ToNot(HaveOccurred())
				Expect(i.Name).To(Equal(intName1))
				addrs, err := i.Addrs()
				Expect(err).ToNot(HaveOccurred())
				subnet1 := s1.IPNet
				subnet1.IP = ip1
				Expect(addrs).To(ContainElements(EqualSubnet(&subnet1)))

				i, err = net.InterfaceByName(intName2)
				Expect(err).ToNot(HaveOccurred())
				Expect(i.Name).To(Equal(intName2))
				addrs, err = i.Addrs()
				Expect(err).ToNot(HaveOccurred())
				subnet2 := s2.IPNet
				subnet2.IP = ip2
				Expect(addrs).To(ContainElements(EqualSubnet(&subnet2)))

				// check loopback adapter
				i, err = net.InterfaceByName("lo")
				Expect(err).ToNot(HaveOccurred())
				Expect(i.Name).To(Equal("lo"))
				Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
				Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			bridgeName1 := network1.NetworkInterface
			// check settings on the host side
			i, err := net.InterfaceByName(bridgeName1)
			Expect(err).ToNot(HaveOccurred())
			Expect(i.Name).To(Equal(bridgeName1))
			addrs, err := i.Addrs()
			Expect(err).ToNot(HaveOccurred())
			// test that the gateway ip is assigned to the interface
			subnet1 := s1.IPNet
			subnet1.IP = gw1
			Expect(addrs).To(ContainElements(EqualSubnet(&subnet1)))

			bridgeName2 := network2.NetworkInterface
			// check settings on the host side
			i, err = net.InterfaceByName(bridgeName2)
			Expect(err).ToNot(HaveOccurred())
			Expect(i.Name).To(Equal(bridgeName2))
			addrs, err = i.Addrs()
			Expect(err).ToNot(HaveOccurred())
			// test that the gateway ip is assigned to the interface
			subnet2 := s2.IPNet
			subnet2.IP = gw2
			Expect(addrs).To(ContainElements(EqualSubnet(&subnet2)))

			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
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

	It("simple teardown", func() {
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

			err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(opts))
			Expect(err).ToNot(HaveOccurred())
			err = netNSContainer.Do(func(_ ns.NetNS) error {
				defer GinkgoRecover()
				// check that the container interface is removed
				_, err := net.InterfaceByName(intName)
				Expect(err).To(HaveOccurred())
				return nil
			})
			Expect(err).ToNot(HaveOccurred())

			// default bridge name
			bridgeName := "podman0"
			// check that bridge interface was removed
			_, err = net.InterfaceByName(bridgeName)
			Expect(err).To(HaveOccurred())
		})
	})

	It("test netavark error", func() {
		runTest(func() {
			intName := "eth0"
			err := netNSContainer.Do(func(_ ns.NetNS) error {
				defer GinkgoRecover()

				attr := netlink.NewLinkAttrs()
				attr.Name = "eth0"
				err := netlink.LinkAdd(&netlink.Bridge{LinkAttrs: attr})
				Expect(err).ToNot(HaveOccurred())
				return nil
			})
			Expect(err).ToNot(HaveOccurred())
			defNet := types.DefaultNetworkName
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
			_, err = libpodNet.Setup(netNSContainer.Path(), opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("interface eth0 already exists on container namespace"))
		})
	})
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
