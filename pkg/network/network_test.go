package network

import (
	"net"
	"testing"
)

func parseCIDR(n string) *net.IPNet {
	_, parsedNet, _ := net.ParseCIDR(n)
	return parsedNet
}

func Test_networkIntersect(t *testing.T) {
	type args struct {
		n1 *net.IPNet
		n2 *net.IPNet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"16 and 24 intersects", args{n1: parseCIDR("192.168.0.0/16"), n2: parseCIDR("192.168.1.0/24")}, true},
		{"24 and 25 intersects", args{n1: parseCIDR("192.168.1.0/24"), n2: parseCIDR("192.168.1.0/25")}, true},
		{"Two 24s", args{n1: parseCIDR("192.168.1.0/24"), n2: parseCIDR("192.168.2.0/24")}, false},
	}
	for _, tt := range tests {
		test := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := networkIntersect(test.args.n1, test.args.n2); got != test.want {
				t.Errorf("networkIntersect() = %v, want %v", got, test.want)
			}
		})
	}
}
