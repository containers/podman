package remoteclientconfig

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

var goodConfig = `
[connections]

[connections.homer]
destination = "192.168.1.1"
username = "myuser"
port = 22
default = true

[connections.bart]
destination = "foobar.com"
username = "root"
port = 22
`
var noDest = `
[connections]

[connections.homer]
destination = "192.168.1.1"
username = "myuser"
default = true
port = 22

[connections.bart]
username = "root"
port = 22
`

var noUser = `
[connections]

[connections.homer]
destination = "192.168.1.1"
port = 22
`

func makeGoodResult() *RemoteConfig {
	var goodConnections = make(map[string]RemoteConnection)
	goodConnections["homer"] = RemoteConnection{
		Destination: "192.168.1.1",
		Username:    "myuser",
		IsDefault:   true,
		Port:        22,
	}
	goodConnections["bart"] = RemoteConnection{
		Destination: "foobar.com",
		Username:    "root",
		Port:        22,
	}
	var goodResult = RemoteConfig{
		Connections: goodConnections,
	}
	return &goodResult
}

func makeNoUserResult() *RemoteConfig {
	var goodConnections = make(map[string]RemoteConnection)
	goodConnections["homer"] = RemoteConnection{
		Destination: "192.168.1.1",
		Port:        22,
	}
	var goodResult = RemoteConfig{
		Connections: goodConnections,
	}
	return &goodResult
}

func TestReadRemoteConfig(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    *RemoteConfig
		wantErr bool
	}{
		// good test should pass
		{"good", args{reader: strings.NewReader(goodConfig)}, makeGoodResult(), false},
		// a connection with no destination is an error
		{"nodest", args{reader: strings.NewReader(noDest)}, nil, true},
		// a connnection with no user is OK
		{"nouser", args{reader: strings.NewReader(noUser)}, makeNoUserResult(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadRemoteConfig(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadRemoteConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadRemoteConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoteConfig_GetDefault(t *testing.T) {
	good := make(map[string]RemoteConnection)
	good["homer"] = RemoteConnection{
		Username:    "myuser",
		Destination: "192.168.1.1",
		IsDefault:   true,
	}
	good["bart"] = RemoteConnection{
		Username:    "root",
		Destination: "foobar.com",
	}
	noDefault := make(map[string]RemoteConnection)
	noDefault["homer"] = RemoteConnection{
		Username:    "myuser",
		Destination: "192.168.1.1",
	}
	noDefault["bart"] = RemoteConnection{
		Username:    "root",
		Destination: "foobar.com",
	}
	single := make(map[string]RemoteConnection)
	single["homer"] = RemoteConnection{
		Username:    "myuser",
		Destination: "192.168.1.1",
	}

	none := make(map[string]RemoteConnection)

	type fields struct {
		Connections map[string]RemoteConnection
	}
	tests := []struct {
		name    string
		fields  fields
		want    *RemoteConnection
		wantErr bool
	}{
		// A good toml should return the connection that is marked isDefault
		{"good", fields{Connections: makeGoodResult().Connections}, &RemoteConnection{"192.168.1.1", "myuser", true, 22, "", false}, false},
		// If nothing is marked as isDefault and there is more than one connection, error should occur
		{"nodefault", fields{Connections: noDefault}, nil, true},
		// if nothing is marked as isDefault but there is only one connection, the one connection is considered the default
		{"single", fields{Connections: none}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RemoteConfig{
				Connections: tt.fields.Connections,
			}
			got, err := r.GetDefault()
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoteConfig.GetDefault() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoteConfig.GetDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoteConfig_GetRemoteConnection(t *testing.T) {
	type fields struct {
		Connections map[string]RemoteConnection
	}
	type args struct {
		name string
	}

	blank := make(map[string]RemoteConnection)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *RemoteConnection
		wantErr bool
	}{
		// Good connection
		{"goodhomer", fields{Connections: makeGoodResult().Connections}, args{name: "homer"}, &RemoteConnection{"192.168.1.1", "myuser", true, 22, "", false}, false},
		// Good connection
		{"goodbart", fields{Connections: makeGoodResult().Connections}, args{name: "bart"}, &RemoteConnection{"foobar.com", "root", false, 22, "", false}, false},
		// Getting an unknown connection should result in error
		{"noexist", fields{Connections: makeGoodResult().Connections}, args{name: "foobar"}, nil, true},
		// Getting a connection when there are none should result in an error
		{"none", fields{Connections: blank}, args{name: "foobar"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RemoteConfig{
				Connections: tt.fields.Connections,
			}
			got, err := r.GetRemoteConnection(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoteConfig.GetRemoteConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoteConfig.GetRemoteConnection() = %v, want %v", got, tt.want)
			}
		})
	}
}
