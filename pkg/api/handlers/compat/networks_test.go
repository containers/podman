//go:build !remote && (linux || freebsd)

package compat

import (
	"net"
	"net/netip"
	"testing"

	nettypes "go.podman.io/common/libnetwork/types"
)

func TestLeaseRangeToIPRangePrefix(t *testing.T) {
	mustPrefix := func(s string) netip.Prefix {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			t.Fatalf("invalid prefix %q: %v", s, err)
		}
		return p
	}
	leaseRange := func(start, end string) *nettypes.LeaseRange {
		return &nettypes.LeaseRange{
			StartIP: net.ParseIP(start),
			EndIP:   net.ParseIP(end),
		}
	}

	tests := []struct {
		name    string
		lr      *nettypes.LeaseRange
		want    netip.Prefix
		wantOK  bool
	}{
		{
			name:   "nil LeaseRange",
			lr:     nil,
			wantOK: false,
		},
		{
			name:   "nil StartIP",
			lr:     &nettypes.LeaseRange{EndIP: net.ParseIP("192.168.1.1")},
			wantOK: false,
		},
		{
			name:   "nil EndIP",
			lr:     &nettypes.LeaseRange{StartIP: net.ParseIP("192.168.1.1")},
			wantOK: false,
		},
		{
			// /24: StartIP = network+1 = .1, EndIP = broadcast = .255
			name:   "/24 subnet",
			lr:     leaseRange("192.168.1.1", "192.168.1.255"),
			want:   mustPrefix("192.168.1.0/24"),
			wantOK: true,
		},
		{
			// /25: StartIP = .129, EndIP = .255
			name:   "/25 subnet",
			lr:     leaseRange("10.10.61.129", "10.10.61.255"),
			want:   mustPrefix("10.10.61.128/25"),
			wantOK: true,
		},
		{
			// /16: StartIP = .0.1, EndIP = .255.255
			name:   "/16 subnet",
			lr:     leaseRange("10.10.0.1", "10.10.255.255"),
			want:   mustPrefix("10.10.0.0/16"),
			wantOK: true,
		},
		{
			// /32: single host, StartIP == EndIP (FirstIPInSubnet returns address unchanged)
			name:   "/32 single host",
			lr:     leaseRange("10.0.0.5", "10.0.0.5"),
			want:   mustPrefix("10.0.0.5/32"),
			wantOK: true,
		},
		{
			// misaligned: StartIP and EndIP do not form a valid CIDR block
			name:   "misaligned range",
			lr:     leaseRange("192.168.1.2", "192.168.1.255"),
			wantOK: false,
		},
		{
			// non-power-of-two size: 3 addresses
			name:   "non-power-of-two size",
			lr:     leaseRange("192.168.1.1", "192.168.1.3"),
			wantOK: false,
		},
		{
			// end before start
			name:   "end before start",
			lr:     leaseRange("192.168.1.5", "192.168.1.1"),
			wantOK: false,
		},
		{
			// IPv6 not supported
			name: "IPv6 not supported",
			lr: &nettypes.LeaseRange{
				StartIP: net.ParseIP("::1"),
				EndIP:   net.ParseIP("::1"),
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := leaseRangeToIPRangePrefix(tt.lr)
			if ok != tt.wantOK {
				t.Errorf("leaseRangeToIPRangePrefix() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && got != tt.want {
				t.Errorf("leaseRangeToIPRangePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
