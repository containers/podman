package specgen

import (
	"net"
	"testing"

	"github.com/containers/common/libnetwork/types"
	"github.com/stretchr/testify/assert"
)

func parsMacNoErr(mac string) types.HardwareAddr {
	m, _ := net.ParseMAC(mac)
	return types.HardwareAddr(m)
}

func TestParseNetworkFlag(t *testing.T) {
	// root and rootless have different defaults
	defaultNetName := "default"

	tests := []struct {
		name     string
		args     []string
		nsmode   Namespace
		networks map[string]types.PerNetworkOptions
		options  map[string][]string
		err      string
	}{
		{
			name:     "empty input",
			args:     nil,
			nsmode:   Namespace{NSMode: Private},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "empty string as input",
			args:     []string{},
			nsmode:   Namespace{NSMode: Private},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "default mode",
			args:     []string{"default"},
			nsmode:   Namespace{NSMode: Private},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "private mode",
			args:     []string{"private"},
			nsmode:   Namespace{NSMode: Private},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:   "bridge mode",
			args:   []string{"bridge"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {},
			},
		},
		{
			name:     "slirp4netns mode",
			args:     []string{"slirp4netns"},
			nsmode:   Namespace{NSMode: Slirp},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "from pod mode",
			args:     []string{"pod"},
			nsmode:   Namespace{NSMode: FromPod},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "no network mode",
			args:     []string{"none"},
			nsmode:   Namespace{NSMode: NoNetwork},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "container mode",
			args:     []string{"container:abc"},
			nsmode:   Namespace{NSMode: FromContainer, Value: "abc"},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "ns path mode",
			args:     []string{"ns:/path"},
			nsmode:   Namespace{NSMode: Path, Value: "/path"},
			networks: map[string]types.PerNetworkOptions{},
		},
		{
			name:     "slirp4netns mode with options",
			args:     []string{"slirp4netns:cidr=10.0.0.0/24"},
			nsmode:   Namespace{NSMode: Slirp},
			networks: map[string]types.PerNetworkOptions{},
			options: map[string][]string{
				"slirp4netns": {"cidr=10.0.0.0/24"},
			},
		},
		{
			name:   "bridge mode with options 1",
			args:   []string{"bridge:ip=10.0.0.1,mac=11:22:33:44:55:66"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					StaticIPs: []net.IP{net.ParseIP("10.0.0.1")},
					StaticMAC: parsMacNoErr("11:22:33:44:55:66"),
				},
			},
		},
		{
			name:   "bridge mode with options 2",
			args:   []string{"bridge:ip=10.0.0.1,ip=10.0.0.5"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					StaticIPs: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.5")},
				},
			},
		},
		{
			name:   "bridge mode with ip6 option",
			args:   []string{"bridge:ip6=fd10::"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					StaticIPs: []net.IP{net.ParseIP("fd10::")},
				},
			},
		},
		{
			name:   "bridge mode with alias option",
			args:   []string{"bridge:alias=myname,alias=myname2"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					Aliases: []string{"myname", "myname2"},
				},
			},
		},
		{
			name:   "bridge mode with alias option",
			args:   []string{"bridge:alias=myname,alias=myname2"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					Aliases: []string{"myname", "myname2"},
				},
			},
		},
		{
			name:   "bridge mode with interface option",
			args:   []string{"bridge:interface_name=eth123"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {
					InterfaceName: "eth123",
				},
			},
		},
		{
			name:   "bridge mode with invalid option",
			args:   []string{"bridge:abc=123"},
			nsmode: Namespace{NSMode: Bridge},
			err:    "unknown bridge network option: abc",
		},
		{
			name:   "bridge mode with invalid ip",
			args:   []string{"bridge:ip=10..1"},
			nsmode: Namespace{NSMode: Bridge},
			err:    "invalid ip address \"10..1\"",
		},
		{
			name:   "bridge mode with invalid mac",
			args:   []string{"bridge:mac=123"},
			nsmode: Namespace{NSMode: Bridge},
			err:    "address 123: invalid MAC address",
		},
		{
			name:   "network name",
			args:   []string{"someName"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				"someName": {},
			},
		},
		{
			name:   "network name with options",
			args:   []string{"someName:ip=10.0.0.1"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				"someName": {StaticIPs: []net.IP{net.ParseIP("10.0.0.1")}},
			},
		},
		{
			name:   "multiple networks",
			args:   []string{"someName", "net2"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				"someName": {},
				"net2":     {},
			},
		},
		{
			name:   "multiple networks with options",
			args:   []string{"someName:ip=10.0.0.1", "net2:ip=10.10.0.1"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				"someName": {StaticIPs: []net.IP{net.ParseIP("10.0.0.1")}},
				"net2":     {StaticIPs: []net.IP{net.ParseIP("10.10.0.1")}},
			},
		},
		{
			name:   "multiple networks with bridge mode first should map to default net",
			args:   []string{"bridge", "net2"},
			nsmode: Namespace{NSMode: Bridge},
			networks: map[string]types.PerNetworkOptions{
				defaultNetName: {},
				"net2":         {},
			},
		},
		{
			name:   "conflicting network modes should error",
			args:   []string{"bridge", "host"},
			nsmode: Namespace{NSMode: Bridge},
			err:    "can only set extra network names, selected mode host conflicts with bridge: invalid argument",
		},
		{
			name:   "multiple networks empty name should error",
			args:   []string{"someName", ""},
			nsmode: Namespace{NSMode: Bridge},
			err:    "network name cannot be empty: invalid argument",
		},
		{
			name:   "multiple networks on invalid mode should error",
			args:   []string{"host", "net2"},
			nsmode: Namespace{NSMode: Host},
			err:    "cannot set multiple networks without bridge network mode, selected mode host: invalid argument",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := ParseNetworkFlag(tt.args)
			if tt.err != "" {
				assert.EqualError(t, err, tt.err, tt.name)
			} else {
				assert.NoError(t, err, tt.name)
			}

			assert.Equal(t, tt.nsmode, got, tt.name)
			assert.Equal(t, tt.networks, got1, tt.name)
			assert.Equal(t, tt.options, got2, tt.name)
		})
	}
}
