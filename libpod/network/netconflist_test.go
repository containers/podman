package network

import (
	"net"
	"reflect"
	"testing"
)

func TestNewIPAMDefaultRoute(t *testing.T) {
	tests := []struct {
		name   string
		isIPv6 bool
		want   IPAMRoute
	}{
		{
			name:   "IPv4 default route",
			isIPv6: false,
			want:   IPAMRoute{defaultIPv4Route},
		},
		{
			name:   "IPv6 default route",
			isIPv6: true,
			want:   IPAMRoute{defaultIPv6Route},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewIPAMDefaultRoute(tt.isIPv6)
			if err != nil {
				t.Errorf("no error expected: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIPAMDefaultRoute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewIPAMLocalHostRange(t *testing.T) {
	tests := []struct {
		name    string
		subnet  *net.IPNet
		ipRange *net.IPNet
		gw      net.IP
		want    []IPAMLocalHostRangeConf
	}{
		{
			name:   "IPv4 subnet",
			subnet: &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			want: []IPAMLocalHostRangeConf{
				{
					Subnet:  "192.168.0.0/24",
					Gateway: "192.168.0.1",
				},
			},
		},
		{
			name:    "IPv4 subnet, range and gateway",
			subnet:  &net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			ipRange: &net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
			gw:      net.ParseIP("192.168.0.10"),
			want: []IPAMLocalHostRangeConf{
				{
					Subnet:     "192.168.0.0/24",
					RangeStart: "192.168.0.129",
					RangeEnd:   "192.168.0.255",
					Gateway:    "192.168.0.10",
				},
			},
		},
		{
			name:   "IPv6 subnet",
			subnet: &net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			want: []IPAMLocalHostRangeConf{
				{
					Subnet:  "2001:db8::/48",
					Gateway: "2001:db8::1",
				},
			},
		},
		{
			name:    "IPv6 subnet, range and gateway",
			subnet:  &net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			ipRange: &net.IPNet{IP: net.ParseIP("2001:DB8:1:1::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff:ffff::"))},
			gw:      net.ParseIP("2001:DB8::2"),
			want: []IPAMLocalHostRangeConf{
				{
					Subnet:     "2001:db8::/48",
					RangeStart: "2001:db8:1:1::1",
					RangeEnd:   "2001:db8:1:1:ffff:ffff:ffff:ffff",
					Gateway:    "2001:db8::2",
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewIPAMLocalHostRange(tt.subnet, tt.ipRange, tt.gw)
			if err != nil {
				t.Errorf("no error expected: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIPAMLocalHostRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
