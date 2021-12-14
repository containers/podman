package netavark

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/podman/v3/libpod/network/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("IPAM", func() {
	var (
		networkInterface *netavarkNetwork
		networkConfDir   string
		logBuffer        bytes.Buffer
	)

	BeforeEach(func() {
		var err error
		networkConfDir, err = ioutil.TempDir("", "podman_netavark_test")
		if err != nil {
			Fail("Failed to create tmpdir")

		}
		logBuffer = bytes.Buffer{}
		logrus.SetOutput(&logBuffer)
	})

	JustBeforeEach(func() {
		libpodNet, err := NewNetworkInterface(InitConfig{
			NetworkConfigDir: networkConfDir,
			IPAMDBPath:       filepath.Join(networkConfDir, "ipam.db"),
			LockFile:         filepath.Join(networkConfDir, "netavark.lock"),
		})
		if err != nil {
			Fail("Failed to create NewCNINetworkInterface")
		}

		networkInterface = libpodNet.(*netavarkNetwork)
		// run network list to force a network load
		networkInterface.NetworkList()
	})

	AfterEach(func() {
		os.RemoveAll(networkConfDir)
	})

	It("simple ipam alloc", func() {
		netName := types.DefaultNetworkName
		for i := 2; i < 100; i++ {
			opts := &types.NetworkOptions{
				ContainerID: "someContainerID",
				Networks: map[string]types.PerNetworkOptions{
					netName: {},
				},
			}

			err := networkInterface.allocIPs(opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Networks).To(HaveKey(netName))
			Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
			Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP(fmt.Sprintf("10.88.0.%d", i)).To4()))
		}
	})

	It("ipam try to alloc same ip", func() {
		netName := types.DefaultNetworkName
		opts := &types.NetworkOptions{
			ContainerID: "someContainerID",
			Networks: map[string]types.PerNetworkOptions{
				netName: {},
			},
		}

		err := networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP("10.88.0.2").To4()))

		opts = &types.NetworkOptions{
			ContainerID: "otherID",
			Networks: map[string]types.PerNetworkOptions{
				netName: {StaticIPs: []net.IP{net.ParseIP("10.88.0.2")}},
			},
		}
		err = networkInterface.allocIPs(opts)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("IPAM error: requested ip address 10.88.0.2 is already allocated to container ID someContainerID"))
	})

	It("ipam try to alloc more ips as in range", func() {
		s, _ := types.ParseCIDR("10.0.0.1/24")
		network, err := networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s,
					LeaseRange: &types.LeaseRange{
						StartIP: net.ParseIP("10.0.0.10"),
						EndIP:   net.ParseIP("10.0.0.20"),
					},
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName := network.Name

		for i := 10; i < 21; i++ {
			opts := &types.NetworkOptions{
				ContainerID: fmt.Sprintf("someContainerID-%d", i),
				Networks: map[string]types.PerNetworkOptions{
					netName: {},
				},
			}

			err = networkInterface.allocIPs(opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Networks).To(HaveKey(netName))
			Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
			Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP(fmt.Sprintf("10.0.0.%d", i)).To4()))
		}

		opts := &types.NetworkOptions{
			ContainerID: "someContainerID-22",
			Networks: map[string]types.PerNetworkOptions{
				netName: {},
			},
		}

		// now this should fail because all free ips are already assigned
		err = networkInterface.allocIPs(opts)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("IPAM error: failed to find free IP in range: 10.0.0.10 - 10.0.0.20"))
	})

	It("ipam basic setup", func() {
		netName := types.DefaultNetworkName
		opts := &types.NetworkOptions{
			ContainerID: "someContainerID",
			Networks: map[string]types.PerNetworkOptions{
				netName: {},
			},
		}

		expectedIP := net.ParseIP("10.88.0.2").To4()

		err := networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(expectedIP))

		// remove static ips from opts
		netOpts := opts.Networks[netName]
		netOpts.StaticIPs = nil
		opts.Networks[netName] = netOpts

		err = networkInterface.getAssignedIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(expectedIP))

		err = networkInterface.allocIPs(opts)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("IPAM error: requested ip address 10.88.0.2 is already allocated to container ID someContainerID"))

		// dealloc the ip
		err = networkInterface.deallocIPs(opts)
		Expect(err).ToNot(HaveOccurred())

		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(expectedIP))
	})

	It("ipam dual stack", func() {
		s1, _ := types.ParseCIDR("10.0.0.0/26")
		s2, _ := types.ParseCIDR("fd80::/24")
		network, err := networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s1,
				},
				{
					Subnet: s2,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName := network.Name

		opts := &types.NetworkOptions{
			ContainerID: "someContainerID",
			Networks: map[string]types.PerNetworkOptions{
				netName: {},
			},
		}

		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(2))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks[netName].StaticIPs[1]).To(Equal(net.ParseIP("fd80::2")))

		// remove static ips from opts
		netOpts := opts.Networks[netName]
		netOpts.StaticIPs = nil
		opts.Networks[netName] = netOpts

		err = networkInterface.getAssignedIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(2))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks[netName].StaticIPs[1]).To(Equal(net.ParseIP("fd80::2")))

		err = networkInterface.deallocIPs(opts)
		Expect(err).ToNot(HaveOccurred())

		// try to alloc the same again
		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(2))
		Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks[netName].StaticIPs[1]).To(Equal(net.ParseIP("fd80::2")))
	})

	It("ipam with two networks", func() {
		s, _ := types.ParseCIDR("10.0.0.0/24")
		network, err := networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName1 := network.Name

		s, _ = types.ParseCIDR("10.0.1.0/24")
		network, err = networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName2 := network.Name

		opts := &types.NetworkOptions{
			ContainerID: "someContainerID",
			Networks: map[string]types.PerNetworkOptions{
				netName1: {},
				netName2: {},
			},
		}

		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName1))
		Expect(opts.Networks[netName1].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName1].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks).To(HaveKey(netName2))
		Expect(opts.Networks[netName2].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName2].StaticIPs[0]).To(Equal(net.ParseIP("10.0.1.2").To4()))

		// remove static ips from opts
		netOpts := opts.Networks[netName1]
		netOpts.StaticIPs = nil
		opts.Networks[netName1] = netOpts
		netOpts = opts.Networks[netName2]
		netOpts.StaticIPs = nil
		opts.Networks[netName2] = netOpts

		err = networkInterface.getAssignedIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName1))
		Expect(opts.Networks[netName1].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName1].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks).To(HaveKey(netName2))
		Expect(opts.Networks[netName2].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName2].StaticIPs[0]).To(Equal(net.ParseIP("10.0.1.2").To4()))

		err = networkInterface.deallocIPs(opts)
		Expect(err).ToNot(HaveOccurred())

		// try to alloc the same again
		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName1))
		Expect(opts.Networks[netName1].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName1].StaticIPs[0]).To(Equal(net.ParseIP("10.0.0.2").To4()))
		Expect(opts.Networks).To(HaveKey(netName2))
		Expect(opts.Networks[netName2].StaticIPs).To(HaveLen(1))
		Expect(opts.Networks[netName2].StaticIPs[0]).To(Equal(net.ParseIP("10.0.1.2").To4()))
	})

	It("ipam alloc more ips as in subnet", func() {
		s, _ := types.ParseCIDR("10.0.0.0/26")
		network, err := networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName := network.Name

		for i := 2; i < 64; i++ {
			opts := &types.NetworkOptions{
				ContainerID: fmt.Sprintf("id-%d", i),
				Networks: map[string]types.PerNetworkOptions{
					netName: {},
				},
			}
			err = networkInterface.allocIPs(opts)
			if i < 63 {
				Expect(err).ToNot(HaveOccurred())
				Expect(opts.Networks).To(HaveKey(netName))
				Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
				Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP(fmt.Sprintf("10.0.0.%d", i)).To4()))
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("IPAM error: failed to find free IP in range: 10.0.0.1 - 10.0.0.62"))
			}
		}
	})

	It("ipam alloc -> dealloc -> alloc", func() {
		s, _ := types.ParseCIDR("10.0.0.0/27")
		network, err := networkInterface.NetworkCreate(types.Network{
			Subnets: []types.Subnet{
				{
					Subnet: s,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName := network.Name

		for i := 2; i < 10; i++ {
			opts := types.NetworkOptions{
				ContainerID: fmt.Sprintf("id-%d", i),
				Networks: map[string]types.PerNetworkOptions{
					netName: {},
				},
			}
			err = networkInterface.allocIPs(&opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Networks).To(HaveKey(netName))
			Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
			Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP(fmt.Sprintf("10.0.0.%d", i)).To4()))

			err = networkInterface.deallocIPs(&opts)
			Expect(err).ToNot(HaveOccurred())
		}

		for i := 0; i < 30; i++ {
			opts := types.NetworkOptions{
				ContainerID: fmt.Sprintf("id-%d", i),
				Networks: map[string]types.PerNetworkOptions{
					netName: {},
				},
			}
			err = networkInterface.allocIPs(&opts)
			if i < 29 {
				Expect(err).ToNot(HaveOccurred())
				Expect(opts.Networks).To(HaveKey(netName))
				Expect(opts.Networks[netName].StaticIPs).To(HaveLen(1))
				// The (i+8)%29+2 part looks cryptic but it is actually simple, we already have 8 ips allocated above
				// so we expect the 8 available ip. We have 29 assignable ip addresses in this subnet because "i"+8 can
				// be greater than 30 we have to modulo by 29 to go back to the beginning. Also the first free ip is
				// network address + 2, so we have to add 2 to the result
				Expect(opts.Networks[netName].StaticIPs[0]).To(Equal(net.ParseIP(fmt.Sprintf("10.0.0.%d", (i+8)%29+2)).To4()))
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("IPAM error: failed to find free IP in range: 10.0.0.1 - 10.0.0.30"))
			}
		}
	})

	It("ipam with dhcp driver should not set ips", func() {
		network, err := networkInterface.NetworkCreate(types.Network{
			IPAMOptions: map[string]string{
				"driver": types.DHCPIPAMDriver,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		netName := network.Name

		opts := &types.NetworkOptions{
			ContainerID: "someContainerID",
			Networks: map[string]types.PerNetworkOptions{
				netName: {},
			},
		}

		err = networkInterface.allocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(0))

		err = networkInterface.getAssignedIPs(opts)
		Expect(err).ToNot(HaveOccurred())
		Expect(opts.Networks).To(HaveKey(netName))
		Expect(opts.Networks[netName].StaticIPs).To(HaveLen(0))

		// dealloc the ip
		err = networkInterface.deallocIPs(opts)
		Expect(err).ToNot(HaveOccurred())
	})

})
