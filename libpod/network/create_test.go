package network

import (
	"net"
	"testing"

	"github.com/containers/podman/v3/pkg/domain/entities"
)

func Test_validateBridgeOptions(t *testing.T) {
	tests := []struct {
		name    string
		subnet  net.IPNet
		ipRange net.IPNet
		gateway net.IP
		isIPv6  bool
		wantErr bool
	}{
		{
			name:   "IPv4 subnet only",
			subnet: net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
		},
		{
			name:    "IPv4 subnet and range",
			subnet:  net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			ipRange: net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
		},
		{
			name:    "IPv4 subnet and gateway",
			subnet:  net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			gateway: net.ParseIP("192.168.0.10"),
		},
		{
			name:    "IPv4 subnet, range and gateway",
			subnet:  net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			ipRange: net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
			gateway: net.ParseIP("192.168.0.10"),
		},
		{
			name:   "IPv6 subnet only",
			subnet: net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
		},
		{
			name:    "IPv6 subnet and range",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			ipRange: net.IPNet{IP: net.ParseIP("2001:DB8:0:0:1::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff:ffff::"))},
			isIPv6:  true,
		},
		{
			name:    "IPv6 subnet and gateway",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			gateway: net.ParseIP("2001:DB8::2"),
			isIPv6:  true,
		},
		{
			name:    "IPv6 subnet, range and gateway",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			ipRange: net.IPNet{IP: net.ParseIP("2001:DB8:0:0:1::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff:ffff::"))},
			gateway: net.ParseIP("2001:DB8::2"),
			isIPv6:  true,
		},
		{
			name:    "IPv6 subnet, range and gateway without IPv6 option (PODMAN SUPPORTS IT UNLIKE DOCKER)",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			ipRange: net.IPNet{IP: net.ParseIP("2001:DB8:0:0:1::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff:ffff::"))},
			gateway: net.ParseIP("2001:DB8::2"),
			isIPv6:  false,
		},
		{
			name:    "range provided but not subnet",
			ipRange: net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
			wantErr: true,
		},
		{
			name:    "gateway provided but not subnet",
			gateway: net.ParseIP("192.168.0.10"),
			wantErr: true,
		},
		{
			name:    "IPv4 subnet but IPv6 required",
			subnet:  net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			ipRange: net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
			gateway: net.ParseIP("192.168.0.10"),
			isIPv6:  true,
			wantErr: true,
		},
		{
			name:    "IPv6 required but IPv4 options used",
			subnet:  net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 255, 0)},
			ipRange: net.IPNet{IP: net.IPv4(192, 168, 0, 128), Mask: net.IPv4Mask(255, 255, 255, 128)},
			gateway: net.ParseIP("192.168.0.10"),
			isIPv6:  true,
			wantErr: true,
		},
		{
			name:    "IPv6 required but not subnet provided",
			isIPv6:  true,
			wantErr: true,
		},
		{
			name:    "range out of the subnet",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			ipRange: net.IPNet{IP: net.ParseIP("2001:1:1::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff:ffff::"))},
			gateway: net.ParseIP("2001:DB8::2"),
			isIPv6:  true,
			wantErr: true,
		},
		{
			name:    "gateway out of the subnet",
			subnet:  net.IPNet{IP: net.ParseIP("2001:DB8::"), Mask: net.IPMask(net.ParseIP("ffff:ffff:ffff::"))},
			gateway: net.ParseIP("2001::2"),
			isIPv6:  true,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			options := entities.NetworkCreateOptions{
				Subnet:  tt.subnet,
				Range:   tt.ipRange,
				Gateway: tt.gateway,
				IPv6:    tt.isIPv6,
			}
			if err := validateBridgeOptions(options); (err != nil) != tt.wantErr {
				t.Errorf("validateBridgeOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
