package util

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/libnetwork/types"
)

func TestValidateRoute(t *testing.T) {
	tests := []struct {
		name        string
		route       types.Route
		wantErr     bool
		errContains string
	}{
		// Valid unicast routes with gateway
		{
			name: "valid unicast route with gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		{
			name: "valid unicast route with empty route type and gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   "",
			},
		},
		{
			name: "valid unicast route with metric",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				Metric:      uint32Ptr(100),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		// Valid blackhole routes without gateway
		{
			name: "valid blackhole route without gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		{
			name: "valid blackhole route with metric",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Metric:      uint32Ptr(200),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		// Valid unreachable routes without gateway
		{
			name: "valid unreachable route without gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "192.168.100.0/24"),
				RouteType:   types.RouteTypeUnreachable,
			},
		},
		{
			name: "valid unreachable route with metric",
			route: types.Route{
				Destination: mustParseCIDR(t, "192.168.100.0/24"),
				Metric:      uint32Ptr(150),
				RouteType:   types.RouteTypeUnreachable,
			},
		},
		// Valid prohibit routes without gateway
		{
			name: "valid prohibit route without gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "172.16.0.0/16"),
				RouteType:   types.RouteTypeProhibit,
			},
		},
		{
			name: "valid prohibit route with metric",
			route: types.Route{
				Destination: mustParseCIDR(t, "172.16.0.0/16"),
				Metric:      uint32Ptr(50),
				RouteType:   types.RouteTypeProhibit,
			},
		},
		// IPv6 routes
		{
			name: "valid unicast IPv6 route",
			route: types.Route{
				Destination: mustParseCIDR(t, "fd00:1::/64"),
				Gateway:     net.ParseIP("fd00::1"),
				RouteType:   types.RouteTypeUnicast,
			},
		},
		{
			name: "valid blackhole IPv6 route",
			route: types.Route{
				Destination: mustParseCIDR(t, "fd00:2::/64"),
				RouteType:   types.RouteTypeBlackhole,
			},
		},
		// Invalid routes - unicast without gateway
		{
			name: "unicast route without gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   types.RouteTypeUnicast,
			},
			wantErr:     true,
			errContains: "unicast route requires gateway",
		},
		{
			name: "empty route type without gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				RouteType:   "",
			},
			wantErr:     true,
			errContains: "unicast route requires gateway",
		},
		// Invalid routes - blackhole with gateway
		{
			name: "blackhole route with gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeBlackhole,
			},
			wantErr:     true,
			errContains: "blackhole route must not have a gateway",
		},
		// Invalid routes - unreachable with gateway
		{
			name: "unreachable route with gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "192.168.100.0/24"),
				Gateway:     net.ParseIP("192.168.1.1"),
				RouteType:   types.RouteTypeUnreachable,
			},
			wantErr:     true,
			errContains: "unreachable route must not have a gateway",
		},
		// Invalid routes - prohibit with gateway
		{
			name: "prohibit route with gateway",
			route: types.Route{
				Destination: mustParseCIDR(t, "172.16.0.0/16"),
				Gateway:     net.ParseIP("172.16.1.1"),
				RouteType:   types.RouteTypeProhibit,
			},
			wantErr:     true,
			errContains: "prohibit route must not have a gateway",
		},
		// Invalid routes - invalid route type
		{
			name: "invalid route type",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.0/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteType("invalid"),
			},
			wantErr:     true,
			errContains: "invalid route type",
		},
		// Invalid routes - nil destination IP
		{
			name: "nil destination IP",
			route: types.Route{
				Destination: types.IPNet{IPNet: net.IPNet{IP: nil, Mask: net.CIDRMask(24, 32)}},
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeUnicast,
			},
			wantErr:     true,
			errContains: "route destination ip nil",
		},
		// Invalid routes - nil destination mask
		{
			name: "nil destination mask",
			route: types.Route{
				Destination: types.IPNet{IPNet: net.IPNet{IP: net.ParseIP("10.21.0.0"), Mask: nil}},
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeUnicast,
			},
			wantErr:     true,
			errContains: "route destination mask nil",
		},
		// Invalid routes - destination is an address not network
		{
			name: "destination is host address not network",
			route: types.Route{
				Destination: mustParseCIDR(t, "10.21.0.100/24"),
				Gateway:     net.ParseIP("10.19.12.250"),
				RouteType:   types.RouteTypeUnicast,
			},
			wantErr:     true,
			errContains: "route destination invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoute(tt.route)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func mustParseCIDR(t *testing.T, cidr string) types.IPNet {
	t.Helper()
	ipnet, err := types.ParseCIDR(cidr)
	require.NoError(t, err)
	return ipnet
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}
