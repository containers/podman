package common

import (
	"net"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func getDefaultNetwork() string {
	if rootless.IsRootless() {
		return "slirp4netns"
	}
	return "bridge"
}

func GetNetFlags() *pflag.FlagSet {
	netFlags := pflag.FlagSet{}
	netFlags.StringSlice(
		"add-host", []string{},
		"Add a custom host-to-IP mapping (host:ip) (default [])",
	)
	netFlags.StringSlice(
		"dns", []string{},
		"Set custom DNS servers",
	)
	netFlags.StringSlice(
		"dns-opt", []string{},
		"Set custom DNS options",
	)
	netFlags.StringSlice(
		"dns-search", []string{},
		"Set custom DNS search domains",
	)
	netFlags.String(
		"ip", "",
		"Specify a static IPv4 address for the container",
	)
	netFlags.String(
		"mac-address", "",
		"Container MAC address (e.g. 92:d0:c6:0a:29:33)",
	)
	netFlags.String(
		"network", getDefaultNetwork(),
		"Connect a container to a network",
	)
	netFlags.StringSliceP(
		"publish", "p", []string{},
		"Publish a container's port, or a range of ports, to the host (default [])",
	)
	netFlags.Bool(
		"no-hosts", false,
		"Do not create /etc/hosts within the container, instead use the version from the image",
	)
	return &netFlags
}

func NetFlagsToNetOptions(cmd *cobra.Command) (*entities.NetOptions, error) {
	var (
		err error
	)
	opts := entities.NetOptions{}
	opts.AddHosts, err = cmd.Flags().GetStringSlice("add-host")
	if err != nil {
		return nil, err
	}
	servers, err := cmd.Flags().GetStringSlice("dns")
	if err != nil {
		return nil, err
	}
	for _, d := range servers {
		if d == "none" {
			opts.DNSHost = true
			break
		}
		opts.DNSServers = append(opts.DNSServers, net.ParseIP(d))
	}
	opts.DNSSearch, err = cmd.Flags().GetStringSlice("dns-search")
	if err != nil {
		return nil, err
	}

	m, err := cmd.Flags().GetString("mac-address")
	if err != nil {
		return nil, err
	}
	if len(m) > 0 {
		mac, err := net.ParseMAC(m)
		if err != nil {
			return nil, err
		}
		opts.StaticMAC = &mac
	}
	inputPorts, err := cmd.Flags().GetStringSlice("publish")
	if err != nil {
		return nil, err
	}
	if len(inputPorts) > 0 {
		opts.PublishPorts, err = createPortBindings(inputPorts)
		if err != nil {
			return nil, err
		}
	}
	opts.NoHosts, err = cmd.Flags().GetBool("no-hosts")
	return &opts, err
}
