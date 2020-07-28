package common

import (
	"net"
	"strings"

	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func GetNetFlags() *pflag.FlagSet {
	netFlags := pflag.FlagSet{}
	netFlags.StringSlice(
		"add-host", []string{},
		"Add a custom host-to-IP mapping (host:ip) (default [])",
	)
	netFlags.StringSlice(
		"dns", containerConfig.DNSServers(),
		"Set custom DNS servers",
	)
	netFlags.StringSlice(
		"dns-opt", containerConfig.DNSOptions(),
		"Set custom DNS options",
	)
	netFlags.StringSlice(
		"dns-search", containerConfig.DNSSearches(),
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
		"network", containerConfig.NetNS(),
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
	// Verify the additional hosts are in correct format
	for _, host := range opts.AddHosts {
		if _, err := parse.ValidateExtraHost(host); err != nil {
			return nil, err
		}
	}

	if cmd.Flags().Changed("dns") {
		servers, err := cmd.Flags().GetStringSlice("dns")
		if err != nil {
			return nil, err
		}
		for _, d := range servers {
			if d == "none" {
				opts.UseImageResolvConf = true
				if len(servers) > 1 {
					return nil, errors.Errorf("%s is not allowed to be specified with other DNS ip addresses", d)
				}
				break
			}
			dns := net.ParseIP(d)
			if dns == nil {
				return nil, errors.Errorf("%s is not an ip address", d)
			}
			opts.DNSServers = append(opts.DNSServers, dns)
		}
	}

	if cmd.Flags().Changed("dns-opt") {
		options, err := cmd.Flags().GetStringSlice("dns-opt")
		if err != nil {
			return nil, err
		}
		opts.DNSOptions = options
	}

	if cmd.Flags().Changed("dns-search") {
		dnsSearches, err := cmd.Flags().GetStringSlice("dns-search")
		if err != nil {
			return nil, err
		}
		// Validate domains are good
		for _, dom := range dnsSearches {
			if dom == "." {
				if len(dnsSearches) > 1 {
					return nil, errors.Errorf("cannot pass additional search domains when also specifying '.'")
				}
				continue
			}
			if _, err := parse.ValidateDomain(dom); err != nil {
				return nil, err
			}
		}
		opts.DNSSearch = dnsSearches
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

	ip, err := cmd.Flags().GetString("ip")
	if err != nil {
		return nil, err
	}
	if ip != "" {
		staticIP := net.ParseIP(ip)
		if staticIP == nil {
			return nil, errors.Errorf("%s is not an ip address", ip)
		}
		if staticIP.To4() == nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "%s is not an IPv4 address", ip)
		}
		opts.StaticIP = &staticIP
	}

	opts.NoHosts, err = cmd.Flags().GetBool("no-hosts")

	if cmd.Flags().Changed("network") {
		network, err := cmd.Flags().GetString("network")
		if err != nil {
			return nil, err
		}

		parts := strings.SplitN(network, ":", 2)

		ns, cniNets, err := specgen.ParseNetworkNamespace(network)
		if err != nil {
			return nil, err
		}

		if len(parts) > 1 {
			opts.NetworkOptions = make(map[string][]string)
			opts.NetworkOptions[parts[0]] = strings.Split(parts[1], ",")
			cniNets = nil
		}
		opts.Network = ns
		opts.CNINetworks = cniNets
	}

	return &opts, err
}
