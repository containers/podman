package network

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
)

// Create makes a new CNI network configuration
func Create(ctx context.Context, network *types.Network) (types.Network, error) {
	var report types.Network
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return report, err
	}
	// create empty network if the caller did not provide one
	if network == nil {
		network = &types.Network{}
	}
	networkConfig, err := jsoniter.MarshalToString(*network)
	if err != nil {
		return report, err
	}
	reader := strings.NewReader(networkConfig)
	response, err := conn.DoRequest(reader, http.MethodPost, "/networks/create", nil, nil)
	if err != nil {
		return report, err
	}
	defer response.Body.Close()

	return report, response.Process(&report)
}

// Inspect returns low level information about a CNI network configuration
func Inspect(ctx context.Context, nameOrID string, _ *InspectOptions) (types.Network, error) {
	var net types.Network
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return net, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/%s/json", nil, nil, nameOrID)
	if err != nil {
		return net, err
	}
	defer response.Body.Close()

	return net, response.Process(&net)
}

// Remove deletes a defined CNI network configuration by name.  The optional force boolean
// will remove all containers associated with the network when set to true.  A slice
// of NetworkRemoveReports are returned.
func Remove(ctx context.Context, nameOrID string, options *RemoveOptions) ([]*entities.NetworkRmReport, error) {
	var reports []*entities.NetworkRmReport
	if options == nil {
		options = new(RemoveOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/networks/%s", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return reports, response.Process(&reports)
}

// List returns a summary of all CNI network configurations
func List(ctx context.Context, options *ListOptions) ([]types.Network, error) {
	var netList []types.Network
	if options == nil {
		options = new(ListOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/json", params, nil)
	if err != nil {
		return netList, err
	}
	defer response.Body.Close()

	return netList, response.Process(&netList)
}

// Disconnect removes a container from a given network
func Disconnect(ctx context.Context, networkName string, ContainerNameOrID string, options *DisconnectOptions) error {
	if options == nil {
		options = new(DisconnectOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	// No params are used for disconnect
	params := url.Values{}
	// Disconnect sends everything in body
	disconnect := struct {
		Container string
		Force     bool
	}{
		Container: ContainerNameOrID,
	}
	if force := options.GetForce(); options.Changed("Force") {
		disconnect.Force = force
	}

	body, err := jsoniter.MarshalToString(disconnect)
	if err != nil {
		return err
	}
	stringReader := strings.NewReader(body)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/networks/%s/disconnect", params, nil, networkName)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return response.Process(nil)
}

// Connect adds a container to a network
func Connect(ctx context.Context, networkName string, ContainerNameOrID string, options *ConnectOptions) error {
	if options == nil {
		options = new(ConnectOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	// No params are used in connect
	params := url.Values{}
	// Connect sends everything in body
	connect := struct {
		Container string
		Aliases   []string
	}{
		Container: ContainerNameOrID,
	}
	if aliases := options.GetAliases(); options.Changed("Aliases") {
		connect.Aliases = aliases
	}
	body, err := jsoniter.MarshalToString(connect)
	if err != nil {
		return err
	}
	stringReader := strings.NewReader(body)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/networks/%s/connect", params, nil, networkName)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return response.Process(nil)
}

// Exists returns true if a given network exists
func Exists(ctx context.Context, nameOrID string, options *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}

// Prune removes unused CNI networks
func Prune(ctx context.Context, options *PruneOptions) ([]*entities.NetworkPruneReport, error) {
	if options == nil {
		options = new(PruneOptions)
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	var (
		prunedNetworks []*entities.NetworkPruneReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodPost, "/networks/prune", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return prunedNetworks, response.Process(&prunedNetworks)
}
