// +build linux

package cni_test

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
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/netns"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
)

var _ = Describe("run CNI", func() {
	var (
		libpodNet      types.ContainerNetwork
		cniConfDir     string
		logBuffer      bytes.Buffer
		netNSTest      ns.NetNS
		netNSContainer ns.NetNS
	)
	const cniVarDir = "/var/lib/cni"

	// runTest is a helper function to run a test. It ensures that each test
	// is run in its own netns. It also creates a mountns to mount a tmpfs to /var/lib/cni.
	runTest := func(run func()) {
		netNSTest.Do(func(_ ns.NetNS) error {
			defer GinkgoRecover()
			err := os.MkdirAll(cniVarDir, 0755)
			Expect(err).To(BeNil(), "Failed to create cniVarDir")
			err = unix.Unshare(unix.CLONE_NEWNS)
			Expect(err).To(BeNil(), "Failed to create new mountns")
			err = unix.Mount("tmpfs", cniVarDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV, "")
			Expect(err).To(BeNil(), "Failed to mount tmpfs for cniVarDir")
			defer unix.Unmount(cniVarDir, 0)

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
		// The tests need root privileges.
		// Technically we could work around that by using user namespaces and
		// the rootless cni code but this is to much work to get it right for a unit test.
		if rootless.IsRootless() {
			Skip("this test needs to be run as root")
		}

		var err error
		cniConfDir, err = ioutil.TempDir("", "podman_cni_test")
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
		libpodNet, err = getNetworkInterface(cniConfDir, false)
		if err != nil {
			Fail("Failed to create NewCNINetworkInterface")
		}
	})

	AfterEach(func() {
		os.RemoveAll(cniConfDir)

		netns.UnmountNS(netNSTest)
		netNSTest.Close()

		netns.UnmountNS(netNSContainer)
		netNSContainer.Close()
	})

	Context("network setup test", func() {

		It("run with default config", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
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

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
			})
		})

		It("run with default config and static ip", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				ip := net.ParseIP("10.88.5.5")
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
								StaticIPs:     []net.IP{ip},
							},
						},
					},
				}
				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(defNet))
				Expect(res[defNet].Interfaces).To(HaveKey(intName))
				Expect(res[defNet].Interfaces[intName].Networks).To(HaveLen(1))
				Expect(res[defNet].Interfaces[intName].Networks[0].Subnet.IP).To(Equal(ip))
				Expect(res[defNet].Interfaces[intName].MacAddress).To(HaveLen(6))
				// default network has no dns
				Expect(res[defNet].DNSServerIPs).To(BeEmpty())
				Expect(res[defNet].DNSSearchDomains).To(BeEmpty())

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
			})
		})

		for _, proto := range []string{"tcp", "udp"} {
			// copy proto to extra var to keep correct references in the goroutines
			protocol := proto
			It("run with exposed ports protocol "+protocol, func() {
				runTest(func() {
					testdata := stringid.GenerateRandomID()
					defNet := types.DefaultNetworkName
					intName := "eth0"
					setupOpts := types.SetupOptions{
						NetworkOptions: types.NetworkOptions{
							ContainerID: stringid.GenerateRandomID(),
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
							ContainerID: stringid.GenerateRandomID(),
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
						testdata := stringid.GenerateRandomID()
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

		It("run with comma separated port protocol", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						PortMappings: []types.PortMapping{{
							Protocol:      "tcp,udp",
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

				for _, proto := range []string{"tcp", "udp"} {
					// copy proto to extra var to keep correct references in the goroutines
					protocol := proto

					testdata := stringid.GenerateRandomID()
					var wg sync.WaitGroup
					wg.Add(1)
					// start tcp listener in the container ns
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
				}

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
			})
		})

		It("call setup twice", func() {
			runTest(func() {
				network := types.Network{}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				intName1 := "eth0"
				netName1 := network1.Name

				containerID := stringid.GenerateRandomID()

				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: containerID,
						Networks: map[string]types.PerNetworkOptions{
							netName1: {
								InterfaceName: intName1,
							},
						},
					},
				}

				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))

				Expect(res).To(HaveKey(netName1))
				Expect(res[netName1].Interfaces).To(HaveKey(intName1))
				Expect(res[netName1].Interfaces[intName1].Networks).To(HaveLen(1))
				ipInt1 := res[netName1].Interfaces[intName1].Networks[0].Subnet.IP
				Expect(ipInt1).ToNot(BeEmpty())
				macInt1 := res[netName1].Interfaces[intName1].MacAddress
				Expect(macInt1).To(HaveLen(6))

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(intName1)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName1))
					Expect(i.HardwareAddr).To(Equal(macInt1))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet := &net.IPNet{
						IP:   ipInt1,
						Mask: net.CIDRMask(24, 32),
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

				network = types.Network{}
				network2, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				intName2 := "eth1"
				netName2 := network2.Name

				setupOpts.Networks = map[string]types.PerNetworkOptions{
					netName2: {
						InterfaceName: intName2,
					},
				}

				res, err = libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))

				Expect(res).To(HaveKey(netName2))
				Expect(res[netName2].Interfaces).To(HaveKey(intName2))
				Expect(res[netName2].Interfaces[intName2].Networks).To(HaveLen(1))
				ipInt2 := res[netName2].Interfaces[intName2].Networks[0].Subnet.IP
				Expect(ipInt2).ToNot(BeEmpty())
				macInt2 := res[netName2].Interfaces[intName2].MacAddress
				Expect(macInt2).To(HaveLen(6))

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(intName1)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName1))
					Expect(i.HardwareAddr).To(Equal(macInt1))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet := &net.IPNet{
						IP:   ipInt1,
						Mask: net.CIDRMask(24, 32),
					}
					Expect(addrs).To(ContainElements(subnet))

					i, err = net.InterfaceByName(intName2)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName2))
					Expect(i.HardwareAddr).To(Equal(macInt2))
					addrs, err = i.Addrs()
					Expect(err).To(BeNil())
					subnet = &net.IPNet{
						IP:   ipInt2,
						Mask: net.CIDRMask(24, 32),
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

				teatdownOpts := types.TeardownOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: containerID,
						Networks: map[string]types.PerNetworkOptions{
							netName1: {
								InterfaceName: intName1,
							},
							netName2: {
								InterfaceName: intName2,
							},
						},
					},
				}

				err = libpodNet.Teardown(netNSContainer.Path(), teatdownOpts)
				Expect(err).To(BeNil())
				logString := logBuffer.String()
				Expect(logString).To(BeEmpty())

				// check in the container namespace that the interface is removed
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					_, err := net.InterfaceByName(intName1)
					Expect(err).To(HaveOccurred())
					_, err = net.InterfaceByName(intName2)
					Expect(err).To(HaveOccurred())

					// check that only the loopback adapter is left
					ints, err := net.Interfaces()
					Expect(err).To(BeNil())
					Expect(ints).To(HaveLen(1))
					Expect(ints[0].Name).To(Equal("lo"))
					Expect(ints[0].Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(ints[0].Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")

					return nil
				})
				Expect(err).To(BeNil())

				err = libpodNet.NetworkRemove(netName1)
				Expect(err).To(BeNil())
				err = libpodNet.NetworkRemove(netName2)
				Expect(err).To(BeNil())

				// check that the interfaces are removed in the host ns
				_, err = net.InterfaceByName(network1.NetworkInterface)
				Expect(err).To(HaveOccurred())
				_, err = net.InterfaceByName(network2.NetworkInterface)
				Expect(err).To(HaveOccurred())
			})
		})

		It("setup two networks with one setup call", func() {
			runTest(func() {
				subnet1, _ := types.ParseCIDR("192.168.0.0/24")
				subnet2, _ := types.ParseCIDR("192.168.1.0/24")
				network := types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet1},
					},
				}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				network = types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet2},
					},
				}
				network2, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				intName1 := "eth0"
				intName2 := "eth1"
				netName1 := network1.Name
				netName2 := network2.Name

				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName1: {
								InterfaceName: intName1,
							},
							netName2: {
								InterfaceName: intName2,
							},
						},
					},
				}

				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))

				Expect(res).To(HaveKey(netName1))
				Expect(res[netName1].Interfaces).To(HaveKey(intName1))
				Expect(res[netName1].Interfaces[intName1].Networks).To(HaveLen(1))
				ipInt1 := res[netName1].Interfaces[intName1].Networks[0].Subnet.IP
				Expect(ipInt1.String()).To(ContainSubstring("192.168.0."))
				macInt1 := res[netName1].Interfaces[intName1].MacAddress
				Expect(macInt1).To(HaveLen(6))

				Expect(res).To(HaveKey(netName2))
				Expect(res[netName2].Interfaces).To(HaveKey(intName2))
				Expect(res[netName2].Interfaces[intName2].Networks).To(HaveLen(1))
				ipInt2 := res[netName2].Interfaces[intName2].Networks[0].Subnet.IP
				Expect(ipInt2.String()).To(ContainSubstring("192.168.1."))
				macInt2 := res[netName2].Interfaces[intName2].MacAddress
				Expect(macInt2).To(HaveLen(6))

				// default network has no dns
				Expect(res[netName1].DNSServerIPs).To(BeEmpty())
				Expect(res[netName1].DNSSearchDomains).To(BeEmpty())

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(intName1)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName1))
					Expect(i.HardwareAddr).To(Equal(macInt1))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet := &net.IPNet{
						IP:   ipInt1,
						Mask: net.CIDRMask(24, 32),
					}
					Expect(addrs).To(ContainElements(subnet))

					i, err = net.InterfaceByName(intName2)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName2))
					Expect(i.HardwareAddr).To(Equal(macInt2))
					addrs, err = i.Addrs()
					Expect(err).To(BeNil())
					subnet = &net.IPNet{
						IP:   ipInt2,
						Mask: net.CIDRMask(24, 32),
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

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
				logString := logBuffer.String()
				Expect(logString).To(BeEmpty())

				// check in the container namespace that the interface is removed
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					_, err := net.InterfaceByName(intName1)
					Expect(err).To(HaveOccurred())
					_, err = net.InterfaceByName(intName2)
					Expect(err).To(HaveOccurred())

					// check that only the loopback adapter is left
					ints, err := net.Interfaces()
					Expect(err).To(BeNil())
					Expect(ints).To(HaveLen(1))
					Expect(ints[0].Name).To(Equal("lo"))
					Expect(ints[0].Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(ints[0].Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")

					return nil
				})
				Expect(err).To(BeNil())
			})

		})

		It("dual stack network with static ips", func() {
			// Version checks for cni plugins are not possible, the plugins do not output
			// version information and using the package manager does not work across distros.
			// Fedora has the right version so we use this for now.
			SkipIfNotFedora("requires cni plugins 1.0.0 or newer for multiple static ips")
			runTest(func() {
				subnet1, _ := types.ParseCIDR("192.168.0.0/24")
				subnet2, _ := types.ParseCIDR("fd41:0a75:2ca0:48a9::/64")
				network := types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet1}, {Subnet: subnet2},
					},
				}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				mac, _ := net.ParseMAC("40:15:2f:d8:42:36")
				interfaceName := "eth0"

				ip1 := net.ParseIP("192.168.0.5")
				ip2 := net.ParseIP("fd41:0a75:2ca0:48a9::5")

				netName := network1.Name
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerName: "mycon",
						ContainerID:   stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName: {
								InterfaceName: interfaceName,
								StaticIPs:     []net.IP{ip1, ip2},
								StaticMAC:     mac,
							},
						},
					},
				}

				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(netName))
				Expect(res[netName].Interfaces).To(HaveKey(interfaceName))
				Expect(res[netName].Interfaces[interfaceName].Networks).To(HaveLen(2))
				Expect(res[netName].Interfaces[interfaceName].Networks[0].Subnet.IP.String()).To(Equal(ip1.String()))
				Expect(res[netName].Interfaces[interfaceName].Networks[0].Subnet.Mask).To(Equal(subnet1.Mask))
				Expect(res[netName].Interfaces[interfaceName].Networks[0].Gateway).To(Equal(net.ParseIP("192.168.0.1")))
				Expect(res[netName].Interfaces[interfaceName].Networks[1].Subnet.IP.String()).To(Equal(ip2.String()))
				Expect(res[netName].Interfaces[interfaceName].Networks[1].Subnet.Mask).To(Equal(subnet2.Mask))
				Expect(res[netName].Interfaces[interfaceName].Networks[1].Gateway).To(Equal(net.ParseIP("fd41:0a75:2ca0:48a9::1")))
				Expect(res[netName].Interfaces[interfaceName].MacAddress).To(Equal(mac))
				// default network has no dns
				Expect(res[netName].DNSServerIPs).To(BeEmpty())
				Expect(res[netName].DNSSearchDomains).To(BeEmpty())

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(interfaceName)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(interfaceName))
					Expect(i.HardwareAddr).To(Equal(mac))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet1 := &net.IPNet{
						IP:   ip1,
						Mask: net.CIDRMask(24, 32),
					}
					subnet2 := &net.IPNet{
						IP:   ip2,
						Mask: net.CIDRMask(64, 128),
					}
					Expect(addrs).To(ContainElements(subnet1, subnet2))

					// check loopback adapter
					i, err = net.InterfaceByName("lo")
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal("lo"))
					Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")
					return nil
				})
				Expect(err).To(BeNil())

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
				logString := logBuffer.String()
				Expect(logString).To(BeEmpty())

				// check in the container namespace that the interface is removed
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					_, err := net.InterfaceByName(interfaceName)
					Expect(err).To(HaveOccurred())

					// check that only the loopback adapter is left
					ints, err := net.Interfaces()
					Expect(err).To(BeNil())
					Expect(ints).To(HaveLen(1))
					Expect(ints[0].Name).To(Equal("lo"))
					Expect(ints[0].Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(ints[0].Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")

					return nil
				})
				Expect(err).To(BeNil())
			})
		})

		It("CNI_ARGS from environment variable", func() {
			runTest(func() {
				subnet1, _ := types.ParseCIDR("172.16.1.0/24")
				ip := "172.16.1.5"
				network := types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet1},
					},
				}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())
				netName := network1.Name
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName: {
								InterfaceName: intName,
							},
						},
					},
				}

				os.Setenv("CNI_ARGS", "IP="+ip)
				defer os.Unsetenv("CNI_ARGS")

				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(netName))
				Expect(res[netName].Interfaces).To(HaveKey(intName))
				Expect(res[netName].Interfaces[intName].Networks).To(HaveLen(1))
				Expect(res[netName].Interfaces[intName].Networks[0].Subnet.IP.String()).To(Equal(ip))
				Expect(res[netName].Interfaces[intName].Networks[0].Subnet.Mask).To(Equal(net.CIDRMask(24, 32)))

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(intName)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(intName))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet := &net.IPNet{
						IP:   net.ParseIP(ip),
						Mask: net.CIDRMask(24, 32),
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
			})
		})
	})

	Context("network setup test with networks from disk", func() {

		BeforeEach(func() {
			dir := "testfiles/valid"
			files, err := ioutil.ReadDir(dir)
			if err != nil {
				Fail("Failed to read test directory")
			}
			for _, file := range files {
				filename := file.Name()
				data, err := ioutil.ReadFile(filepath.Join(dir, filename))
				if err != nil {
					Fail("Failed to copy test files")
				}
				err = ioutil.WriteFile(filepath.Join(cniConfDir, filename), data, 0700)
				if err != nil {
					Fail("Failed to copy test files")
				}
			}
		})

		It("dualstack setup with static ip and dns", func() {
			SkipIfNoDnsname()
			// Version checks for cni plugins are not possible, the plugins do not output
			// version information and using the package manager does not work across distros.
			// Fedora has the right version so we use this for now.
			SkipIfNotFedora("requires cni plugins 1.0.0 or newer for multiple static ips")
			runTest(func() {
				interfaceName := "eth0"

				ip1 := net.ParseIP("fd10:88:a::11")
				ip2 := net.ParseIP("10.89.19.15")

				containerName := "myname"
				aliases := []string{"aliasname"}

				netName := "dualstack"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID:   stringid.GenerateRandomID(),
						ContainerName: containerName,
						Networks: map[string]types.PerNetworkOptions{
							netName: {
								InterfaceName: interfaceName,
								StaticIPs:     []net.IP{ip1, ip2},
								Aliases:       aliases,
							},
						},
					},
				}

				network, err := libpodNet.NetworkInspect(netName)
				Expect(err).To(BeNil())
				Expect(network.Name).To(Equal(netName))
				Expect(network.DNSEnabled).To(BeTrue())
				Expect(network.Subnets).To(HaveLen(2))
				gw1 := network.Subnets[0].Gateway
				Expect(gw1).To(HaveLen(16))
				mask1 := network.Subnets[0].Subnet.Mask
				Expect(mask1).To(HaveLen(16))
				gw2 := network.Subnets[1].Gateway
				Expect(gw2).To(HaveLen(4))
				mask2 := network.Subnets[1].Subnet.Mask
				Expect(mask2).To(HaveLen(4))

				// because this net has dns we should always teardown otherwise we leak a dnsmasq process
				defer libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				res, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
				Expect(res).To(HaveKey(netName))
				Expect(res[netName].Interfaces).To(HaveKey(interfaceName))
				Expect(res[netName].Interfaces[interfaceName].Networks).To(HaveLen(2))
				Expect(res[netName].Interfaces[interfaceName].Networks[0].Subnet.IP.String()).To(Equal(ip1.String()))
				Expect(res[netName].Interfaces[interfaceName].Networks[0].Subnet.Mask).To(Equal(mask1))
				Expect(res[netName].Interfaces[interfaceName].Networks[1].Subnet.IP.String()).To(Equal(ip2.String()))
				Expect(res[netName].Interfaces[interfaceName].Networks[1].Subnet.Mask).To(Equal(mask2))
				// dualstack network dns
				Expect(res[netName].DNSServerIPs).To(HaveLen(2))
				Expect(res[netName].DNSSearchDomains).To(HaveLen(1))
				Expect(res[netName].DNSSearchDomains).To(ConsistOf("dns.podman"))

				// check in the container namespace if the settings are applied
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					i, err := net.InterfaceByName(interfaceName)
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal(interfaceName))
					addrs, err := i.Addrs()
					Expect(err).To(BeNil())
					subnet1 := &net.IPNet{
						IP:   ip1,
						Mask: net.CIDRMask(64, 128),
					}
					subnet2 := &net.IPNet{
						IP:   ip2,
						Mask: net.CIDRMask(24, 32),
					}
					Expect(addrs).To(ContainElements(subnet1, subnet2))

					// check loopback adapter
					i, err = net.InterfaceByName("lo")
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal("lo"))
					Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")

					return nil
				})
				Expect(err).To(BeNil())

				err = libpodNet.Teardown(netNSContainer.Path(), types.TeardownOptions(setupOpts))
				Expect(err).To(BeNil())
				logString := logBuffer.String()
				Expect(logString).To(BeEmpty())

				// check in the container namespace that the interface is removed
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					_, err := net.InterfaceByName(interfaceName)
					Expect(err).To(HaveOccurred())

					// check that only the loopback adapter is left
					ints, err := net.Interfaces()
					Expect(err).To(BeNil())
					Expect(ints).To(HaveLen(1))
					Expect(ints[0].Name).To(Equal("lo"))
					Expect(ints[0].Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(ints[0].Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")

					return nil
				})
				Expect(err).To(BeNil())
			})
		})

	})

	Context("invalid network setup test", func() {

		It("static ip not in subnet", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				ip := "1.1.1.1"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
								StaticIPs:     []net.IP{net.ParseIP(ip)},
							},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("requested static ip %s not in any subnet on network %s", ip, defNet))
			})
		})

		It("setup without namespace path", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
							},
						},
					},
				}
				_, err := libpodNet.Setup("", setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("namespacePath is empty"))
			})
		})

		It("setup with invalid namespace path", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
							},
						},
					},
				}
				_, err := libpodNet.Setup("some path", setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(`"some path": no such file or directory`))
			})
		})

		It("setup without container ID", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: "",
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
							},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ContainerID is empty"))
			})
		})

		It("setup with aliases but dns disabled", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: intName,
								Aliases:       []string{"somealias"},
							},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot set aliases on a network without dns enabled"))
			})
		})

		It("setup without networks", func() {
			runTest(func() {
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must specify at least one network"))
			})
		})

		It("setup without interface name", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {
								InterfaceName: "",
							},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("interface name on network %s is empty", defNet))
			})
		})

		It("setup does teardown on failure", func() {
			runTest(func() {
				subnet1, _ := types.ParseCIDR("192.168.0.0/24")
				network := types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet1},
					},
				}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				subnet2, _ := types.ParseCIDR("192.168.1.0/31")
				network = types.Network{
					Subnets: []types.Subnet{
						{Subnet: subnet2},
					},
				}
				network2, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				intName1 := "eth0"
				intName2 := "eth1"
				netName1 := network1.Name
				netName2 := network2.Name

				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName1: {
								InterfaceName: intName1,
							},
							netName2: {
								InterfaceName: intName2,
							},
						},
					},
				}
				_, err = libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Network 192.168.1.0/31 too small to allocate from"))
				// Note: we call teardown on the failing net and log the error, it should be the same.
				logString := logBuffer.String()
				Expect(logString).To(ContainSubstring("Network 192.168.1.0/31 too small to allocate from"))

				// check in the container namespace that no interface is there
				err = netNSContainer.Do(func(_ ns.NetNS) error {
					defer GinkgoRecover()
					_, err := net.InterfaceByName(intName1)
					Expect(err).To(HaveOccurred())

					// Note: We can check if intName2 is removed because
					// the cni plugin fails before it removes the interface

					// check loopback adapter
					i, err := net.InterfaceByName("lo")
					Expect(err).To(BeNil())
					Expect(i.Name).To(Equal("lo"))
					Expect(i.Flags & net.FlagLoopback).To(Equal(net.FlagLoopback))
					Expect(i.Flags&net.FlagUp).To(Equal(net.FlagUp), "Loopback adapter should be up")
					return nil
				})
				Expect(err).To(BeNil())
			})
		})

		It("setup with exposed invalid port protocol", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						PortMappings: []types.PortMapping{{
							Protocol:      "someproto",
							HostIP:        "127.0.0.1",
							HostPort:      5000,
							ContainerPort: 5000,
						}},
						Networks: map[string]types.PerNetworkOptions{
							defNet: {InterfaceName: intName},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown port protocol someproto"))
			})
		})

		It("setup with exposed empty port protocol", func() {
			runTest(func() {
				defNet := types.DefaultNetworkName
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						PortMappings: []types.PortMapping{{
							Protocol:      "",
							HostIP:        "127.0.0.1",
							HostPort:      5000,
							ContainerPort: 5000,
						}},
						Networks: map[string]types.PerNetworkOptions{
							defNet: {InterfaceName: intName},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("port protocol should not be empty"))
			})
		})

		It("setup with unknown network", func() {
			runTest(func() {
				defNet := "somenet"
				intName := "eth0"
				setupOpts := types.SetupOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							defNet: {InterfaceName: intName},
						},
					},
				}
				_, err := libpodNet.Setup(netNSContainer.Path(), setupOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("network somenet: network not found"))
			})
		})

		It("teardown with unknown network", func() {
			runTest(func() {
				interfaceName := "eth0"
				netName := "somenet"
				teardownOpts := types.TeardownOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName: {
								InterfaceName: interfaceName,
							},
						},
					},
				}

				err := libpodNet.Teardown(netNSContainer.Path(), teardownOpts)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("network somenet: network not found"))
				logString := logBuffer.String()
				Expect(logString).To(ContainSubstring("failed to load cached network config"))
			})
		})

		It("teardown on not connected network", func() {
			runTest(func() {
				network := types.Network{}
				network1, err := libpodNet.NetworkCreate(network)
				Expect(err).To(BeNil())

				interfaceName := "eth0"
				netName := network1.Name
				teardownOpts := types.TeardownOptions{
					NetworkOptions: types.NetworkOptions{
						ContainerID: stringid.GenerateRandomID(),
						Networks: map[string]types.PerNetworkOptions{
							netName: {
								InterfaceName: interfaceName,
							},
						},
					},
				}

				// Most CNI plugins do not error on teardown when there is nothing to do.
				err = libpodNet.Teardown(netNSContainer.Path(), teardownOpts)
				Expect(err).To(BeNil())
				logString := logBuffer.String()
				Expect(logString).To(ContainSubstring("failed to load cached network config"))
			})
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
			conn, err := ln.Accept()
			Expect(err).To(BeNil())
			conn.SetDeadline(time.Now().Add(1 * time.Second))
			data, err := ioutil.ReadAll(conn)
			Expect(err).To(BeNil())
			Expect(string(data)).To(Equal(expectedData))
			conn.Close()
			ln.Close()
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
			data := make([]byte, len(expectedData))
			i, err := conn.Read(data)
			Expect(err).To(BeNil())
			Expect(i).To(Equal(len(expectedData)))
			Expect(string(data)).To(Equal(expectedData))
			conn.Close()
		}()
	default:
		Fail("unsupported protocol")
	}
}
