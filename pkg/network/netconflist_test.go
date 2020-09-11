package network

import (
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
