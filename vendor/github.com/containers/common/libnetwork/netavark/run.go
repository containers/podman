//go:build linux || freebsd
// +build linux freebsd

package netavark

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/common/libnetwork/internal/util"
	"github.com/containers/common/libnetwork/types"
	pkgutil "github.com/containers/common/pkg/util"
	"github.com/sirupsen/logrus"
)

type netavarkOptions struct {
	types.NetworkOptions
	Networks map[string]*types.Network `json:"network_info"`
}

func (n *netavarkNetwork) execUpdate(networkName string, networkDNSServers []string) error {
	retErr := n.execNetavark([]string{"update", networkName, "--network-dns-servers", strings.Join(networkDNSServers, ",")}, false, nil, nil)
	return retErr
}

// Setup will setup the container network namespace. It returns
// a map of StatusBlocks, the key is the network name.
func (n *netavarkNetwork) Setup(namespacePath string, options types.SetupOptions) (map[string]types.StatusBlock, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return nil, err
	}

	err = util.ValidateSetupOptions(n, namespacePath, options)
	if err != nil {
		return nil, err
	}

	// allocate IPs in the IPAM db
	err = n.allocIPs(&options.NetworkOptions)
	if err != nil {
		return nil, err
	}

	netavarkOpts, needPlugin, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert net opts: %w", err)
	}

	// Warn users if one or more networks have dns enabled
	// but aardvark-dns binary is not configured
	for _, network := range netavarkOpts.Networks {
		if network != nil && network.DNSEnabled && n.aardvarkBinary == "" {
			// this is not a fatal error we can still use container without dns
			logrus.Warnf("aardvark-dns binary not found, container dns will not be enabled")
			break
		}
	}

	// trace output to get the json
	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		b, err := json.Marshal(&netavarkOpts)
		if err != nil {
			return nil, err
		}
		// show the full netavark command so we can easily reproduce errors from the cli
		logrus.Tracef("netavark command: printf '%s' | %s setup %s", string(b), n.netavarkBinary, namespacePath)
	}

	result := map[string]types.StatusBlock{}
	err = n.execNetavark([]string{"setup", namespacePath}, needPlugin, netavarkOpts, &result)
	if err != nil {
		// lets dealloc ips to prevent leaking
		if err := n.deallocIPs(&options.NetworkOptions); err != nil {
			logrus.Error(err)
		}
		return nil, err
	}

	// make sure that the result makes sense
	if len(result) != len(options.Networks) {
		logrus.Errorf("unexpected netavark result: %v", result)
		return nil, fmt.Errorf("unexpected netavark result length, want (%d), got (%d) networks", len(options.Networks), len(result))
	}

	return result, err
}

// Teardown will teardown the container network namespace.
func (n *netavarkNetwork) Teardown(namespacePath string, options types.TeardownOptions) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}

	// get IPs from the IPAM db
	err = n.getAssignedIPs(&options.NetworkOptions)
	if err != nil {
		// when there is an error getting the ips we should still continue
		// to call teardown for netavark to prevent leaking network interfaces
		logrus.Error(err)
	}

	netavarkOpts, needPlugin, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return fmt.Errorf("failed to convert net opts: %w", err)
	}

	retErr := n.execNetavark([]string{"teardown", namespacePath}, needPlugin, netavarkOpts, nil)

	// when netavark returned an error we still free the used ips
	// otherwise we could end up in a state where block the ips forever
	err = n.deallocIPs(&netavarkOpts.NetworkOptions)
	if err != nil {
		if retErr != nil {
			logrus.Error(err)
		} else {
			retErr = err
		}
	}

	return retErr
}

func (n *netavarkNetwork) getCommonNetavarkOptions(needPlugin bool) []string {
	opts := []string{"--config", n.networkRunDir, "--rootless=" + strconv.FormatBool(n.networkRootless), "--aardvark-binary=" + n.aardvarkBinary}
	// to allow better backwards compat we only add the new netavark option when really needed
	if needPlugin {
		// Note this will require a netavark with https://github.com/containers/netavark/pull/509
		for _, dir := range n.pluginDirs {
			opts = append(opts, "--plugin-directory", dir)
		}
	}
	return opts
}

func (n *netavarkNetwork) convertNetOpts(opts types.NetworkOptions) (*netavarkOptions, bool, error) {
	netavarkOptions := netavarkOptions{
		NetworkOptions: opts,
		Networks:       make(map[string]*types.Network, len(opts.Networks)),
	}

	needsPlugin := false

	for network := range opts.Networks {
		net, err := n.getNetwork(network)
		if err != nil {
			return nil, false, err
		}
		netavarkOptions.Networks[network] = net
		if !pkgutil.StringInSlice(net.Driver, builtinDrivers) {
			needsPlugin = true
		}
	}
	return &netavarkOptions, needsPlugin, nil
}
