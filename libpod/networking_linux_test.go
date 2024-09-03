//go:build !remote

package libpod

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v5/libpod/define"
)

func Test_ocicniPortsToNetTypesPorts(t *testing.T) {
	tests := []struct {
		name string
		arg  []types.OCICNIPortMapping
		want []types.PortMapping
	}{
		{
			name: "no ports",
			arg:  nil,
			want: nil,
		},
		{
			name: "empty ports",
			arg:  []types.OCICNIPortMapping{},
			want: nil,
		},
		{
			name: "single port",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "two separate ports",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      9000,
					ContainerPort: 90,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      9000,
					ContainerPort: 90,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "two ports joined",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         2,
				},
			},
		},
		{
			name: "three ports with different container port are not joined",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      8081,
					ContainerPort: 79,
					Protocol:      "tcp",
				},
				{
					HostPort:      8082,
					ContainerPort: 82,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8081,
					ContainerPort: 79,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8082,
					ContainerPort: 82,
					Protocol:      "tcp",
					Range:         1,
				},
			},
		},
		{
			name: "three ports joined (not sorted)",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
				},
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      8082,
					ContainerPort: 82,
					Protocol:      "tcp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         3,
				},
			},
		},
		{
			name: "different protocols ports are not joined",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "udp",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "udp",
					Range:         1,
				},
			},
		},
		{
			name: "different host ip ports are not joined",
			arg: []types.OCICNIPortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					HostIP:        "192.168.1.1",
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
					HostIP:        "192.168.1.2",
				},
			},
			want: []types.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 80,
					Protocol:      "tcp",
					Range:         1,
					HostIP:        "192.168.1.1",
				},
				{
					HostPort:      8081,
					ContainerPort: 81,
					Protocol:      "tcp",
					Range:         1,
					HostIP:        "192.168.1.2",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ocicniPortsToNetTypesPorts(tt.arg)
			assert.Equal(t, tt.want, result, "ports do not match")
		})
	}
}

func Test_resultToBasicNetworkConfig(t *testing.T) {
	testCases := []struct {
		description           string
		inputResult           types.StatusBlock
		expectedNetworkConfig define.InspectBasicNetworkConfig
	}{
		{
			description: "single secondary IPv4 address is shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("172.26.0.1"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.2"),
										Mask: net.CIDRMask(20, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.3"),
										Mask: net.CIDRMask(10, 32),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				IPAddress:   "172.26.0.2",
				IPPrefixLen: 20,
				Gateway:     "172.26.0.1",
				SecondaryIPAddresses: []define.Address{
					{
						Addr:         "172.26.0.3",
						PrefixLength: 10,
					},
				},
			},
		},
		{
			description: "multiple secondary IPv4 addresses are shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("172.26.0.1"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.2"),
										Mask: net.CIDRMask(20, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.3"),
										Mask: net.CIDRMask(10, 32),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("172.26.0.4"),
										Mask: net.CIDRMask(24, 32),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				IPAddress:   "172.26.0.2",
				IPPrefixLen: 20,
				Gateway:     "172.26.0.1",
				SecondaryIPAddresses: []define.Address{
					{
						Addr:         "172.26.0.3",
						PrefixLength: 10,
					},
					{
						Addr:         "172.26.0.4",
						PrefixLength: 24,
					},
				},
			},
		},
		{
			description: "single secondary IPv6 address is shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("ff02::fb"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fc"),
										Mask: net.CIDRMask(20, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fd"),
										Mask: net.CIDRMask(10, 128),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				GlobalIPv6Address:   "ff02::fc",
				GlobalIPv6PrefixLen: 20,
				IPv6Gateway:         "ff02::fb",
				SecondaryIPv6Addresses: []define.Address{
					{
						Addr:         "ff02::fd",
						PrefixLength: 10,
					},
				},
			},
		},
		{
			description: "multiple secondary IPv6 addresses are shown as define.Address",
			inputResult: types.StatusBlock{
				Interfaces: map[string]types.NetInterface{
					"eth1": {
						Subnets: []types.NetAddress{
							{
								Gateway: net.ParseIP("ff02::fb"),
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fc"),
										Mask: net.CIDRMask(20, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fd"),
										Mask: net.CIDRMask(10, 128),
									},
								},
							},
							{
								IPNet: types.IPNet{
									IPNet: net.IPNet{
										IP:   net.ParseIP("ff02::fe"),
										Mask: net.CIDRMask(24, 128),
									},
								},
							},
						},
					},
				},
			},
			expectedNetworkConfig: define.InspectBasicNetworkConfig{
				GlobalIPv6Address:   "ff02::fc",
				GlobalIPv6PrefixLen: 20,
				IPv6Gateway:         "ff02::fb",
				SecondaryIPv6Addresses: []define.Address{
					{
						Addr:         "ff02::fd",
						PrefixLength: 10,
					},
					{
						Addr:         "ff02::fe",
						PrefixLength: 24,
					},
				},
			},
		},
	}

	for _, tcl := range testCases {
		tc := tcl
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			actualNetworkConfig := resultToBasicNetworkConfig(tc.inputResult)

			if !reflect.DeepEqual(tc.expectedNetworkConfig, actualNetworkConfig) {
				t.Fatalf(
					"Expected networkConfig %+v didn't match actual value %+v", tc.expectedNetworkConfig, actualNetworkConfig)
			}
		})
	}
}

func benchmarkOCICNIPortsToNetTypesPorts(b *testing.B, ports []types.OCICNIPortMapping) {
	for n := 0; n < b.N; n++ {
		ocicniPortsToNetTypesPorts(ports)
	}
}

func Benchmark_ocicniPortsToNetTypesPortsNoPorts(b *testing.B) {
	benchmarkOCICNIPortsToNetTypesPorts(b, nil)
}

func Benchmark_ocicniPortsToNetTypesPorts1(b *testing.B) {
	benchmarkOCICNIPortsToNetTypesPorts(b, []types.OCICNIPortMapping{
		{
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
		},
	})
}

func Benchmark_ocicniPortsToNetTypesPorts10(b *testing.B) {
	ports := make([]types.OCICNIPortMapping, 0, 10)
	for i := int32(8080); i < 8090; i++ {
		ports = append(ports, types.OCICNIPortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkOCICNIPortsToNetTypesPorts(b, ports)
}

func Benchmark_ocicniPortsToNetTypesPorts100(b *testing.B) {
	ports := make([]types.OCICNIPortMapping, 0, 100)
	for i := int32(8080); i < 8180; i++ {
		ports = append(ports, types.OCICNIPortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkOCICNIPortsToNetTypesPorts(b, ports)
}

func Benchmark_ocicniPortsToNetTypesPorts1k(b *testing.B) {
	ports := make([]types.OCICNIPortMapping, 0, 1000)
	for i := int32(8080); i < 9080; i++ {
		ports = append(ports, types.OCICNIPortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkOCICNIPortsToNetTypesPorts(b, ports)
}

func Benchmark_ocicniPortsToNetTypesPorts10k(b *testing.B) {
	ports := make([]types.OCICNIPortMapping, 0, 30000)
	for i := int32(8080); i < 18080; i++ {
		ports = append(ports, types.OCICNIPortMapping{
			HostPort:      i,
			ContainerPort: i,
			Protocol:      "tcp",
		})
	}
	b.ResetTimer()
	benchmarkOCICNIPortsToNetTypesPorts(b, ports)
}

func Benchmark_ocicniPortsToNetTypesPorts1m(b *testing.B) {
	ports := make([]types.OCICNIPortMapping, 0, 1000000)
	for j := 0; j < 20; j++ {
		for i := int32(1); i <= 50000; i++ {
			ports = append(ports, types.OCICNIPortMapping{
				HostPort:      i,
				ContainerPort: i,
				Protocol:      "tcp",
				HostIP:        fmt.Sprintf("192.168.1.%d", j),
			})
		}
	}
	b.ResetTimer()
	benchmarkOCICNIPortsToNetTypesPorts(b, ports)
}
