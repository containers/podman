// +build linux

package netavark

import (
	"encoding/json"
	"fmt"

	"github.com/containers/podman/v3/libpod/network/internal/util"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type netavarkOptions struct {
	types.NetworkOptions
	Networks map[string]*types.Network `json:"network_info"`
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

	// TODO IP address assignment

	netavarkOpts, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert net opts")
	}

	b, err := json.Marshal(&netavarkOpts)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(b))

	result := map[string]types.StatusBlock{}
	err = execNetavark(n.netavarkBinary, []string{"setup", namespacePath}, netavarkOpts, result)

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

	netavarkOpts, err := n.convertNetOpts(options.NetworkOptions)
	if err != nil {
		return errors.Wrap(err, "failed to convert net opts")
	}

	return execNetavark(n.netavarkBinary, []string{"teardown", namespacePath}, netavarkOpts, nil)
}

func (n *netavarkNetwork) convertNetOpts(opts types.NetworkOptions) (*netavarkOptions, error) {
	netavarkOptions := netavarkOptions{
		NetworkOptions: opts,
		Networks:       make(map[string]*types.Network, len(opts.Networks)),
	}

	for network := range opts.Networks {
		net, err := n.getNetwork(network)
		if err != nil {
			return nil, err
		}
		netavarkOptions.Networks[network] = net
	}
	return &netavarkOptions, nil
}
