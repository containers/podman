//nolint
// most of these validate and parse functions have been taken from projectatomic/docker
// and modified for cri-o
package parse

import (
	"testing"
)

func TestValidateExtraHost(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		//2001:0db8:85a3:0000:0000:8a2e:0370:7334
		{name: "good-ipv4", args: args{val: "foobar:192.168.1.1"}, want: "foobar:192.168.1.1", wantErr: false},
		{name: "bad-ipv4", args: args{val: "foobar:999.999.999.99"}, want: "", wantErr: true},
		{name: "bad-ipv4", args: args{val: "foobar:999.999.999"}, want: "", wantErr: true},
		{name: "noname-ipv4", args: args{val: "192.168.1.1"}, want: "", wantErr: true},
		{name: "noname-ipv4", args: args{val: ":192.168.1.1"}, want: "", wantErr: true},
		{name: "noip", args: args{val: "foobar:"}, want: "", wantErr: true},
		{name: "noip", args: args{val: "foobar"}, want: "", wantErr: true},
		{name: "good-ipv6", args: args{val: "foobar:2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "foobar:2001:0db8:85a3:0000:0000:8a2e:0370:7334", wantErr: false},
		{name: "bad-ipv6", args: args{val: "foobar:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "bad-ipv6", args: args{val: "foobar:0db8:85a3:0000:0000:8a2e:0370:7334.0000.0000.000"}, want: "", wantErr: true},
		{name: "noname-ipv6", args: args{val: "2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "noname-ipv6", args: args{val: ":2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateExtraHost(tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtraHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateExtraHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateIPAddress(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "ipv4-good", args: args{val: "192.168.1.1"}, want: "192.168.1.1", wantErr: false},
		{name: "ipv4-bad", args: args{val: "192.168.1.1.1"}, want: "", wantErr: true},
		{name: "ipv4-bad", args: args{val: "192."}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateIPAddress(tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIPAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateIPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFileName(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "good", args: args{filename: "/som/rand/path"}, wantErr: false},
		{name: "good", args: args{filename: "som/rand/path"}, wantErr: false},
		{name: "good", args: args{filename: "/"}, wantErr: false},
		{name: "bad", args: args{filename: "/:"}, wantErr: true},
		{name: "bad", args: args{filename: ":/"}, wantErr: true},
		{name: "bad", args: args{filename: "/some/rand:/path"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateFileName(tt.args.filename); (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
