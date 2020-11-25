package network

import (
	"net"
	"reflect"
	"testing"
)

func TestNextSubnet(t *testing.T) {
	type args struct {
		subnet *net.IPNet
	}
	tests := []struct {
		name    string
		args    args
		want    *net.IPNet
		wantErr bool
	}{
		{"class b", args{subnet: parseCIDR("192.168.0.0/16")}, parseCIDR("192.169.0.0/16"), false},
		{"class c", args{subnet: parseCIDR("192.168.1.0/24")}, parseCIDR("192.168.2.0/24"), false},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			got, err := NextSubnet(test.args.subnet)
			if (err != nil) != test.wantErr {
				t.Errorf("NextSubnet() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("NextSubnet() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFirstIPInSubnet(t *testing.T) {
	tests := []struct {
		name    string
		args    *net.IPNet
		want    net.IP
		wantErr bool
	}{
		{"class b", parseCIDR("192.168.0.0/16"), net.ParseIP("192.168.0.1"), false},
		{"class c", parseCIDR("192.168.1.0/24"), net.ParseIP("192.168.1.1"), false},
		{"cidr /23", parseCIDR("192.168.0.0/23"), net.ParseIP("192.168.0.1"), false},
		{"cidr /25", parseCIDR("192.168.1.0/25"), net.ParseIP("192.168.1.1"), false},
		{"cidr /26", parseCIDR("172.16.1.128/26"), net.ParseIP("172.16.1.129"), false},
		{"class a", parseCIDR("10.0.0.0/8"), net.ParseIP("10.0.0.1"), false},
		{"cidr /32", parseCIDR("192.168.255.4/32"), net.ParseIP("192.168.255.4"), false},
		{"cidr /31", parseCIDR("192.168.255.4/31"), net.ParseIP("192.168.255.5"), false},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			got, err := FirstIPInSubnet(test.args)
			if (err != nil) != test.wantErr {
				t.Errorf("FirstIPInSubnet() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !got.Equal(test.want) {
				t.Errorf("FirstIPInSubnet() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLastIPInSubnet(t *testing.T) {
	tests := []struct {
		name    string
		args    *net.IPNet
		want    net.IP
		wantErr bool
	}{
		{"class b", parseCIDR("192.168.0.0/16"), net.ParseIP("192.168.255.255"), false},
		{"class c", parseCIDR("192.168.1.0/24"), net.ParseIP("192.168.1.255"), false},
		{"cidr /23", parseCIDR("192.168.0.0/23"), net.ParseIP("192.168.1.255"), false},
		{"cidr /25", parseCIDR("192.168.1.0/25"), net.ParseIP("192.168.1.127"), false},
		{"cidr /26", parseCIDR("172.16.1.128/26"), net.ParseIP("172.16.1.191"), false},
		{"class a", parseCIDR("10.0.0.0/8"), net.ParseIP("10.255.255.255"), false},
		{"cidr /32", parseCIDR("192.168.255.4/32"), net.ParseIP("192.168.255.4"), false},
		{"cidr /31", parseCIDR("192.168.255.4/31"), net.ParseIP("192.168.255.5"), false},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			got, err := LastIPInSubnet(test.args)
			if (err != nil) != test.wantErr {
				t.Errorf("LastIPInSubnet() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !got.Equal(test.want) {
				t.Errorf("LastIPInSubnet() got = %v, want %v", got, test.want)
			}
		})
	}
}
