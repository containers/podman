// +build linux

package netavark_test

import (
	"bytes"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegaTypes "github.com/onsi/gomega/types"
	"github.com/sirupsen/logrus"

	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
)

var _ = Describe("Config", func() {
	var (
		libpodNet      types.ContainerNetwork
		networkConfDir string
		logBuffer      bytes.Buffer
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
		var err error
		libpodNet, err = getNetworkInterface(networkConfDir, false)
		if err != nil {
			Fail("Failed to create NewCNINetworkInterface")
		}
	})

	AfterEach(func() {
		os.RemoveAll(networkConfDir)
	})

	Context("basic network config tests", func() {

		It("check default network config exists", func() {
			networks, err := libpodNet.NetworkList()
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(1))
			Expect(networks[0].Name).To(Equal("podman"))
			Expect(networks[0].Driver).To(Equal("bridge"))
			Expect(networks[0].ID).To(Equal("2f259bab93aaaaa2542ba43ef33eb990d0999ee1b9924b557b7be53c0b7a1bb9"))
			Expect(networks[0].NetworkInterface).To(Equal("podman0"))
			Expect(networks[0].Created.Before(time.Now())).To(BeTrue())
			Expect(networks[0].Subnets).To(HaveLen(1))
			Expect(networks[0].Subnets[0].Subnet.String()).To(Equal("10.88.0.0/16"))
			Expect(networks[0].Subnets[0].Gateway.String()).To(Equal("10.88.0.1"))
			Expect(networks[0].Subnets[0].LeaseRange).To(BeNil())
			Expect(networks[0].IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
			Expect(networks[0].Options).To(BeEmpty())
			Expect(networks[0].Labels).To(BeEmpty())
			Expect(networks[0].DNSEnabled).To(BeFalse())
			Expect(networks[0].Internal).To(BeFalse())
		})

		It("basic network create, inspect and remove", func() {
			// Because we get the time from the file create timestamp there is small precision
			// loss so lets remove 500 milliseconds to make sure this test does not flake.
			now := time.Now().Add(-500 * time.Millisecond)
			network := types.Network{}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Labels).To(BeEmpty())
			Expect(network1.Options).To(BeEmpty())
			Expect(network1.IPAMOptions).ToNot(BeEmpty())
			Expect(network1.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
			Expect(network1.Created.After(now)).To(BeTrue())
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal("10.89.0.0/24"))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.89.0.1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.DNSEnabled).To(BeFalse())
			Expect(network1.Internal).To(BeFalse())

			// inspect by name
			network2, err := libpodNet.NetworkInspect(network1.Name)
			Expect(err).To(BeNil())
			EqualNetwork(network2, network1)

			// inspect by ID
			network2, err = libpodNet.NetworkInspect(network1.ID)
			Expect(err).To(BeNil())
			EqualNetwork(network2, network1)

			// inspect by partial ID
			network2, err = libpodNet.NetworkInspect(network1.ID[:10])
			Expect(err).To(BeNil())
			EqualNetwork(network2, network1)

			// create a new interface to force a config load from disk
			libpodNet, err = getNetworkInterface(networkConfDir, false)
			Expect(err).To(BeNil())

			network2, err = libpodNet.NetworkInspect(network1.Name)
			Expect(err).To(BeNil())
			EqualNetwork(network2, network1)

			err = libpodNet.NetworkRemove(network1.Name)
			Expect(err).To(BeNil())
			Expect(path).ToNot(BeARegularFile())

			_, err = libpodNet.NetworkInspect(network1.Name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("network not found"))
		})

		It("create two networks", func() {
			network := types.Network{}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.Subnets).To(HaveLen(1))

			network = types.Network{}
			network2, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network2.Name).ToNot(Equal(network1.Name))
			Expect(network2.ID).ToNot(Equal(network1.ID))
			Expect(network2.NetworkInterface).ToNot(Equal(network1.NetworkInterface))
			Expect(network2.Subnets).To(HaveLen(1))
			Expect(network2.Subnets[0].Subnet.Contains(network1.Subnets[0].Subnet.IP)).To(BeFalse())
		})

		It("create bridge config", func() {
			network := types.Network{Driver: "bridge"}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(filepath.Join(networkConfDir, network1.Name+".json")).To(BeARegularFile())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Labels).To(BeEmpty())
			Expect(network1.Options).To(BeEmpty())
			Expect(network1.IPAMOptions).ToNot(BeEmpty())
			Expect(network1.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal("10.89.0.0/24"))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.89.0.1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.DNSEnabled).To(BeFalse())
			Expect(network1.Internal).To(BeFalse())
		})

		It("create bridge with same name should fail", func() {
			network := types.Network{
				Driver:           "bridge",
				NetworkInterface: "podman2",
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).To(Equal("podman2"))
			Expect(network1.Driver).To(Equal("bridge"))

			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bridge name podman2 already in use"))
		})

		It("create bridge with subnet", func() {
			subnet := "10.0.0.0/24"
			n, _ := types.ParseCIDR(subnet)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.0.0.1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
		})

		It("create bridge with ipv6 subnet", func() {
			subnet := "fdcc::/64"
			n, _ := types.ParseCIDR(subnet)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.IPv6Enabled).To(BeTrue())
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("fdcc::1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())

			// reload configs from disk
			libpodNet, err = getNetworkInterface(networkConfDir, false)
			Expect(err).To(BeNil())
			// check the the networks are identical
			network2, err := libpodNet.NetworkInspect(network1.Name)
			Expect(err).To(BeNil())
			EqualNetwork(network2, network1)
		})

		It("create bridge with ipv6 enabled", func() {
			network := types.Network{
				Driver:      "bridge",
				IPv6Enabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(2))
			Expect(network1.Subnets[0].Subnet.String()).To(ContainSubstring(".0/24"))
			Expect(network1.Subnets[0].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.Subnets[1].Subnet.String()).To(ContainSubstring("::/64"))
			Expect(network1.Subnets[1].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[1].LeaseRange).To(BeNil())
		})

		It("create bridge with ipv6 enabled and ipv4 subnet", func() {
			subnet := "10.100.0.0/24"
			n, _ := types.ParseCIDR(subnet)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
				IPv6Enabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(2))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.Subnets[1].Subnet.String()).To(ContainSubstring("::/64"))
			Expect(network1.Subnets[1].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[1].LeaseRange).To(BeNil())
		})

		It("create bridge with ipv6 enabled and ipv6 subnet", func() {
			subnet := "fd66::/64"
			n, _ := types.ParseCIDR(subnet)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
				IPv6Enabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(2))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.Subnets[1].Subnet.String()).To(ContainSubstring(".0/24"))
			Expect(network1.Subnets[1].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[1].LeaseRange).To(BeNil())
		})

		It("create bridge with ipv6 enabled and ipv4+ipv6 subnet", func() {
			subnet1 := "10.100.0.0/24"
			n1, _ := types.ParseCIDR(subnet1)
			subnet2 := "fd66::/64"
			n2, _ := types.ParseCIDR(subnet2)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n1}, {Subnet: n2},
				},
				IPv6Enabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(2))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet1))
			Expect(network1.Subnets[0].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.Subnets[1].Subnet.String()).To(Equal(subnet2))
			Expect(network1.Subnets[1].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[1].LeaseRange).To(BeNil())
		})

		It("create bridge with ipv6 enabled and two ipv4 subnets", func() {
			subnet1 := "10.100.0.0/24"
			n1, _ := types.ParseCIDR(subnet1)
			subnet2 := "10.200.0.0/24"
			n2, _ := types.ParseCIDR(subnet2)

			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n1}, {Subnet: n2},
				},
				IPv6Enabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(3))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet1))
			Expect(network1.Subnets[0].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.Subnets[1].Subnet.String()).To(Equal(subnet2))
			Expect(network1.Subnets[1].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[1].LeaseRange).To(BeNil())
			Expect(network1.Subnets[2].Subnet.String()).To(ContainSubstring("::/64"))
			Expect(network1.Subnets[2].Gateway).ToNot(BeNil())
			Expect(network1.Subnets[2].LeaseRange).To(BeNil())
		})

		It("create bridge with subnet and gateway", func() {
			subnet := "10.0.0.5/24"
			n, _ := types.ParseCIDR(subnet)
			gateway := "10.0.0.50"
			g := net.ParseIP(gateway)
			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, Gateway: g},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal("10.0.0.0/24"))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal(gateway))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
		})

		It("create bridge with subnet and gateway not in the same subnet", func() {
			subnet := "10.0.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			gateway := "10.10.0.50"
			g := net.ParseIP(gateway)
			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, Gateway: g},
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in subnet"))
		})

		It("create bridge with subnet and lease range", func() {
			subnet := "10.0.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			startIP := "10.0.0.10"
			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, LeaseRange: &types.LeaseRange{
						StartIP: net.ParseIP(startIP),
					}},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.0.0.1"))
			Expect(network1.Subnets[0].LeaseRange.StartIP.String()).To(Equal(startIP))

			err = libpodNet.NetworkRemove(network1.Name)
			Expect(err).To(BeNil())

			endIP := "10.0.0.10"
			network = types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, LeaseRange: &types.LeaseRange{
						EndIP: net.ParseIP(endIP),
					}},
				},
			}
			network1, err = libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(filepath.Join(networkConfDir, network1.Name+".json")).To(BeARegularFile())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.0.0.1"))
			Expect(network1.Subnets[0].LeaseRange.EndIP.String()).To(Equal(endIP))

			err = libpodNet.NetworkRemove(network1.Name)
			Expect(err).To(BeNil())

			network = types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, LeaseRange: &types.LeaseRange{
						StartIP: net.ParseIP(startIP),
						EndIP:   net.ParseIP(endIP),
					}},
				},
			}
			network1, err = libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.NetworkInterface).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.0.0.1"))
			Expect(network1.Subnets[0].LeaseRange.StartIP.String()).To(Equal(startIP))
			Expect(network1.Subnets[0].LeaseRange.EndIP.String()).To(Equal(endIP))
		})

		It("create bridge with subnet and invalid lease range", func() {
			subnet := "10.0.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			startIP := "10.0.1.2"
			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, LeaseRange: &types.LeaseRange{
						StartIP: net.ParseIP(startIP),
					}},
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in subnet"))

			endIP := "10.1.1.1"
			network = types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: n, LeaseRange: &types.LeaseRange{
						EndIP: net.ParseIP(endIP),
					}},
				},
			}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in subnet"))
		})

		It("create bridge with broken subnet", func() {
			network := types.Network{
				Driver: "bridge",
				Subnets: []types.Subnet{
					{Subnet: types.IPNet{}},
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("subnet ip is nil"))
		})

		It("create network with name", func() {
			name := "myname"
			network := types.Network{
				Name: name,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).To(Equal(name))
			Expect(network1.NetworkInterface).ToNot(Equal(name))
			Expect(network1.Driver).To(Equal("bridge"))
		})

		It("create network with invalid name", func() {
			name := "myname@some"
			network := types.Network{
				Name: name,
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
		})

		It("create network with name", func() {
			name := "myname"
			network := types.Network{
				Name: name,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).To(Equal(name))
			Expect(network1.NetworkInterface).ToNot(Equal(name))
			Expect(network1.Driver).To(Equal("bridge"))
		})

		It("create network with invalid name", func() {
			name := "myname@some"
			network := types.Network{
				Name: name,
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
		})

		It("create network with interface name", func() {
			name := "myname"
			network := types.Network{
				NetworkInterface: name,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(Equal(name))
			Expect(network1.NetworkInterface).To(Equal(name))
			Expect(network1.Driver).To(Equal("bridge"))
		})

		It("create network with invalid interface name", func() {
			name := "myname@some"
			network := types.Network{
				NetworkInterface: name,
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
		})

		It("create network with ID should fail", func() {
			id := "17f29b073143d8cd97b5bbe492bdeffec1c5fee55cc1fe2112c8b9335f8b6121"
			network := types.Network{
				ID: id,
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ID can not be set for network create"))
		})

		It("create bridge with dns", func() {
			network := types.Network{
				Driver:     "bridge",
				DNSEnabled: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.DNSEnabled).To(BeTrue())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			grepInFile(path, `"dns_enabled": true`)
		})

		It("create bridge with internal", func() {
			network := types.Network{
				Driver:   "bridge",
				Internal: true,
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).ToNot(BeEmpty())
			Expect(network1.Subnets[0].Gateway).To(BeNil())
			Expect(network1.Internal).To(BeTrue())
		})

		It("create network with labels", func() {
			network := types.Network{
				Labels: map[string]string{
					"key": "value",
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Labels).ToNot(BeNil())
			Expect(network1.Labels).To(ContainElement("value"))
		})

		It("create network with mtu option", func() {
			network := types.Network{
				Options: map[string]string{
					"mtu": "1500",
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Options).ToNot(BeNil())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			grepInFile(path, `"mtu": "1500"`)
			Expect(network1.Options).To(HaveKeyWithValue("mtu", "1500"))
		})

		It("create network with invalid mtu option", func() {
			network := types.Network{
				Options: map[string]string{
					"mtu": "abc",
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`parsing "abc": invalid syntax`))

			network = types.Network{
				Options: map[string]string{
					"mtu": "-1",
				},
			}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`mtu -1 is less than zero`))
		})

		It("create network with vlan option", func() {
			network := types.Network{
				Options: map[string]string{
					"vlan": "5",
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Driver).To(Equal("bridge"))
			Expect(network1.Options).ToNot(BeNil())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			grepInFile(path, `"vlan": "5"`)
			Expect(network1.Options).To(HaveKeyWithValue("vlan", "5"))
		})

		It("create network with invalid vlan option", func() {
			network := types.Network{
				Options: map[string]string{
					"vlan": "abc",
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`parsing "abc": invalid syntax`))

			network = types.Network{
				Options: map[string]string{
					"vlan": "-1",
				},
			}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`vlan ID -1 must be between 0 and 4094`))
		})

		It("network create unsupported option", func() {
			network := types.Network{Options: map[string]string{
				"someopt": "",
			}}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported bridge network option someopt"))
		})

		It("network create unsupported driver", func() {
			network := types.Network{
				Driver: "someDriver",
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported driver someDriver"))
		})

		It("network create internal and dns", func() {
			network := types.Network{
				Driver:     "bridge",
				Internal:   true,
				DNSEnabled: true,
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot set internal and dns enabled"))
		})

		It("network inspect partial ID", func() {
			network := types.Network{Name: "net4"}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.ID).To(HaveLen(64))

			network2, err := libpodNet.NetworkInspect(network1.ID[:10])
			Expect(err).ToNot(HaveOccurred())
			EqualNetwork(network2, network1)
		})

		It("network create two with same name", func() {
			network := types.Network{Name: "net"}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).To(Equal("net"))
			network = types.Network{Name: "net"}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("network name net already used"))
		})

		It("remove default network config should fail", func() {
			err := libpodNet.NetworkRemove("podman")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("default network podman cannot be removed"))

			network, err := libpodNet.NetworkInspect("podman")
			Expect(err).To(BeNil())
			err = libpodNet.NetworkRemove(network.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("default network podman cannot be removed"))
		})

		It("network create with same subnet", func() {
			subnet := "10.0.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			subnet2 := "10.10.0.0/24"
			n2, _ := types.ParseCIDR(subnet2)
			network := types.Network{Subnets: []types.Subnet{{Subnet: n}, {Subnet: n2}}}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Subnets).To(HaveLen(2))
			network = types.Network{Subnets: []types.Subnet{{Subnet: n}}}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("subnet 10.0.0.0/24 is already used on the host or by another config"))
			network = types.Network{Subnets: []types.Subnet{{Subnet: n2}}}
			_, err = libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("subnet 10.10.0.0/24 is already used on the host or by another config"))
		})

		It("create macvlan config without subnet", func() {
			network := types.Network{Driver: "macvlan"}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("macvlan driver needs at least one subnet specified, DHCP is not supported with netavark"))
		})

		It("create macvlan config with internal", func() {
			network := types.Network{Driver: "macvlan", Internal: true}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("internal is not supported with macvlan"))
		})

		It("create macvlan config with subnet", func() {
			subnet := "10.1.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			network := types.Network{
				Driver: "macvlan",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("macvlan"))
			Expect(network1.NetworkInterface).To(Equal(""))
			Expect(network1.Labels).To(BeEmpty())
			Expect(network1.Options).To(BeEmpty())
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.1.0.1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.DNSEnabled).To(BeFalse())
			Expect(network1.Internal).To(BeFalse())
			Expect(network1.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
		})

		It("create macvlan config with subnet and device", func() {
			subnet := "10.1.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			network := types.Network{
				Driver:           "macvlan",
				NetworkInterface: "lo",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			path := filepath.Join(networkConfDir, network1.Name+".json")
			Expect(path).To(BeARegularFile())
			Expect(network1.ID).ToNot(BeEmpty())
			Expect(network1.Driver).To(Equal("macvlan"))
			Expect(network1.NetworkInterface).To(Equal("lo"))
			Expect(network1.Labels).To(BeEmpty())
			Expect(network1.Options).To(BeEmpty())
			Expect(network1.Subnets).To(HaveLen(1))
			Expect(network1.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(network1.Subnets[0].Gateway.String()).To(Equal("10.1.0.1"))
			Expect(network1.Subnets[0].LeaseRange).To(BeNil())
			Expect(network1.DNSEnabled).To(BeFalse())
			Expect(network1.Internal).To(BeFalse())
			Expect(network1.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
		})

		It("create macvlan config with mode", func() {
			subnet := "10.1.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			network := types.Network{
				Driver: "macvlan",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
				Options: map[string]string{
					"mode": "private",
				},
			}
			network1, err := libpodNet.NetworkCreate(network)
			Expect(err).To(BeNil())
			Expect(network1.Name).ToNot(BeEmpty())
			Expect(network1.Options).To(HaveKeyWithValue("mode", "private"))
		})

		It("create macvlan config with invalid mode", func() {
			subnet := "10.1.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			network := types.Network{
				Driver: "macvlan",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
				Options: map[string]string{
					"mode": "abc",
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unknown macvlan mode \"abc\""))
		})

		It("create macvlan config with invalid option", func() {
			subnet := "10.1.0.0/24"
			n, _ := types.ParseCIDR(subnet)
			network := types.Network{
				Driver: "macvlan",
				Subnets: []types.Subnet{
					{Subnet: n},
				},
				Options: map[string]string{
					"abc": "123",
				},
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported macvlan network option abc"))
		})

	})

	Context("network load valid existing ones", func() {

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
				err = ioutil.WriteFile(filepath.Join(networkConfDir, filename), data, 0700)
				if err != nil {
					Fail("Failed to copy test files")
				}
			}
		})

		It("load networks from disk", func() {
			nets, err := libpodNet.NetworkList()
			Expect(err).To(BeNil())
			Expect(nets).To(HaveLen(7))
			// test the we do not show logrus warnings/errors
			logString := logBuffer.String()
			Expect(logString).To(BeEmpty())
		})

		It("change network struct fields should not affect network struct in the backend", func() {
			nets, err := libpodNet.NetworkList()
			Expect(err).To(BeNil())
			Expect(nets).To(HaveLen(7))

			nets[0].Name = "myname"
			nets, err = libpodNet.NetworkList()
			Expect(err).To(BeNil())
			Expect(nets).To(HaveLen(7))
			Expect(nets).ToNot(ContainElement(HaveNetworkName("myname")))

			network, err := libpodNet.NetworkInspect("bridge")
			Expect(err).To(BeNil())
			network.NetworkInterface = "abc"

			network, err = libpodNet.NetworkInspect("bridge")
			Expect(err).To(BeNil())
			Expect(network.NetworkInterface).ToNot(Equal("abc"))
		})

		It("bridge network", func() {
			network, err := libpodNet.NetworkInspect("bridge")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("bridge"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman9"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(1))
			Expect(network.Subnets[0].Subnet.String()).To(Equal("10.89.8.0/24"))
			Expect(network.Subnets[0].Gateway.String()).To(Equal("10.89.8.1"))
			Expect(network.Subnets[0].LeaseRange).ToNot(BeNil())
			Expect(network.Subnets[0].LeaseRange.StartIP.String()).To(Equal("10.89.8.20"))
			Expect(network.Subnets[0].LeaseRange.EndIP.String()).To(Equal("10.89.8.50"))
			Expect(network.Internal).To(BeFalse())
		})

		It("internal network", func() {
			network, err := libpodNet.NetworkInspect("internal")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("internal"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman8"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(1))
			Expect(network.Subnets[0].Subnet.String()).To(Equal("10.89.7.0/24"))
			Expect(network.Subnets[0].Gateway).To(BeNil())
			Expect(network.Internal).To(BeTrue())
		})

		It("bridge network with mtu", func() {
			network, err := libpodNet.NetworkInspect("mtu")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("mtu"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman13"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(1))
			Expect(network.Subnets[0].Subnet.String()).To(Equal("10.89.11.0/24"))
			Expect(network.Subnets[0].Gateway.String()).To(Equal("10.89.11.1"))
			Expect(network.Internal).To(BeFalse())
			Expect(network.Options).To(HaveLen(1))
			Expect(network.Options).To(HaveKeyWithValue("mtu", "1500"))
		})

		It("bridge network with vlan", func() {
			network, err := libpodNet.NetworkInspect("vlan")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("vlan"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman14"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(1))
			Expect(network.Options).To(HaveLen(1))
			Expect(network.Options).To(HaveKeyWithValue("vlan", "5"))
		})

		It("bridge network with labels", func() {
			network, err := libpodNet.NetworkInspect("label")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("label"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman15"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(1))
			Expect(network.Labels).To(HaveLen(1))
			Expect(network.Labels).To(HaveKeyWithValue("mykey", "value"))
		})

		It("dual stack network", func() {
			network, err := libpodNet.NetworkInspect("dualstack")
			Expect(err).To(BeNil())
			Expect(network.Name).To(Equal("dualstack"))
			Expect(network.ID).To(HaveLen(64))
			Expect(network.NetworkInterface).To(Equal("podman21"))
			Expect(network.Driver).To(Equal("bridge"))
			Expect(network.Subnets).To(HaveLen(2))

			sub1, _ := types.ParseCIDR("fd10:88:a::/64")
			sub2, _ := types.ParseCIDR("10.89.19.0/24")
			Expect(network.Subnets).To(ContainElements(
				types.Subnet{Subnet: sub1, Gateway: net.ParseIP("fd10:88:a::1")},
				types.Subnet{Subnet: sub2, Gateway: net.ParseIP("10.89.19.10").To4()},
			))
		})

		It("network list with filters (name)", func() {
			filters := map[string][]string{
				"name": {"internal", "bridge"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(2))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge")))
		})

		It("network list with filters (partial name)", func() {
			filters := map[string][]string{
				"name": {"inte", "bri"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(2))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge")))
		})

		It("network list with filters (id)", func() {
			filters := map[string][]string{
				"id": {"3bed2cb3a3acf7b6a8ef408420cc682d5520e26976d354254f528c965612054f", "17f29b073143d8cd97b5bbe492bdeffec1c5fee55cc1fe2112c8b9335f8b6121"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(2))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge")))
		})

		It("network list with filters (id)", func() {
			filters := map[string][]string{
				"id": {"3bed2cb3a3acf7b6a8ef408420cc682d5520e26976d354254f528c965612054f", "17f29b073143d8cd97b5bbe492bdeffec1c5fee55cc1fe2112c8b9335f8b6121"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(2))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge")))
		})

		It("network list with filters (partial id)", func() {
			filters := map[string][]string{
				"id": {"3bed2cb3a3acf7b6a8ef408420", "17f29b073143d8cd97b5bbe492bde"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(2))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge")))
		})

		It("network list with filters (driver)", func() {
			filters := map[string][]string{
				"driver": {"bridge"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(7))
			Expect(networks).To(ConsistOf(HaveNetworkName("internal"), HaveNetworkName("bridge"),
				HaveNetworkName("mtu"), HaveNetworkName("vlan"), HaveNetworkName("podman"),
				HaveNetworkName("label"), HaveNetworkName("dualstack")))
		})

		It("network list with filters (label)", func() {
			filters := map[string][]string{
				"label": {"mykey"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(1))
			Expect(networks).To(ConsistOf(HaveNetworkName("label")))

			filters = map[string][]string{
				"label": {"mykey=value"},
			}
			filterFuncs, err = util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err = libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(1))
			Expect(networks).To(ConsistOf(HaveNetworkName("label")))
		})

		It("network list with filters", func() {
			filters := map[string][]string{
				"driver": {"bridge"},
				"label":  {"mykey"},
			}
			filterFuncs, err := util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())
			Expect(filterFuncs).To(HaveLen(2))

			networks, err := libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(1))
			Expect(networks).To(ConsistOf(HaveNetworkName("label")))

			filters = map[string][]string{
				"driver": {"macvlan"},
				"label":  {"mykey"},
			}
			filterFuncs, err = util.GenerateNetworkFilters(filters)
			Expect(err).To(BeNil())

			networks, err = libpodNet.NetworkList(filterFuncs...)
			Expect(err).To(BeNil())
			Expect(networks).To(HaveLen(0))
		})

		It("create bridge network with used interface name", func() {
			network := types.Network{
				NetworkInterface: "podman9",
			}
			_, err := libpodNet.NetworkCreate(network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bridge name podman9 already in use"))
		})
	})

	Context("network load invalid existing ones", func() {

		BeforeEach(func() {
			dir := "testfiles/invalid"
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
				err = ioutil.WriteFile(filepath.Join(networkConfDir, filename), data, 0700)
				if err != nil {
					Fail("Failed to copy test files")
				}
			}
		})

		It("load invalid networks from disk", func() {
			nets, err := libpodNet.NetworkList()
			Expect(err).To(BeNil())
			Expect(nets).To(HaveLen(1))
			logString := logBuffer.String()
			Expect(logString).To(ContainSubstring("Error reading network config file \\\"%s/broken.json\\\": unexpected EOF", networkConfDir))
			Expect(logString).To(ContainSubstring("Network config \\\"%s/invalid name.json\\\" has invalid name: \\\"invalid name\\\", skipping: names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*: invalid argument", networkConfDir))
			Expect(logString).To(ContainSubstring("Network config name \\\"name_miss\\\" does not match file name \\\"name_missmatch.json\\\", skipping"))
			Expect(logString).To(ContainSubstring("Network config \\\"%s/wrongID.json\\\" could not be parsed, skipping: invalid network ID \\\"someID\\\"", networkConfDir))
			Expect(logString).To(ContainSubstring("Network config \\\"%s/invalid_gateway.json\\\" could not be parsed, skipping: gateway 10.89.100.1 not in subnet 10.89.9.0/24", networkConfDir))
		})

	})

})

func grepInFile(path string, match string) {
	data, err := ioutil.ReadFile(path)
	ExpectWithOffset(1, err).To(BeNil())
	ExpectWithOffset(1, string(data)).To(ContainSubstring(match))
}

// HaveNetworkName is a custom GomegaMatcher to match a network name
func HaveNetworkName(name string) gomegaTypes.GomegaMatcher {
	return WithTransform(func(e types.Network) string {
		return e.Name
	}, Equal(name))
}

// EqualNetwork must be used because comparing the time with deep equal does not work
func EqualNetwork(net1, net2 types.Network) {
	ExpectWithOffset(1, net1.Created.Equal(net2.Created)).To(BeTrue(), "net1 created: %v is not equal net2 created: %v", net1.Created, net2.Created)
	net1.Created = time.Time{}
	net2.Created = time.Time{}
	ExpectWithOffset(1, net1).To(Equal(net2))
}
