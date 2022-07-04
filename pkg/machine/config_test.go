//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"net"
	"net/url"
	"reflect"
	"testing"
)

func TestRemoteConnectionType_MakeSSHURL(t *testing.T) {
	var (
		host     = "foobar"
		path     = "/path/to/socket"
		rc       = "ssh"
		username = "core"
	)
	type args struct {
		host     string
		path     string
		port     string
		userName string
	}
	tests := []struct {
		name string
		rc   RemoteConnectionType
		args args
		want url.URL
	}{
		{
			name: "Good no port",
			rc:   "ssh",
			args: args{
				host:     host,
				path:     path,
				port:     "",
				userName: username,
			},
			want: url.URL{
				Scheme:     rc,
				User:       url.User(username),
				Host:       host,
				Path:       path,
				ForceQuery: false,
			},
		},
		{
			name: "Good with port",
			rc:   "ssh",
			args: args{
				host:     host,
				path:     path,
				port:     "222",
				userName: username,
			},
			want: url.URL{
				Scheme:     rc,
				User:       url.User(username),
				Host:       net.JoinHostPort(host, "222"),
				Path:       path,
				ForceQuery: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rc.MakeSSHURL(tt.args.host, tt.args.path, tt.args.port, tt.args.userName); !reflect.DeepEqual(got, tt.want) { //nolint: scopelint
				t.Errorf("MakeSSHURL() = %v, want %v", got, tt.want) //nolint: scopelint
			}
		})
	}
}
