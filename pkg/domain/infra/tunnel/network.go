package tunnel

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/network"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/errorhandling"
)

func (ic *ContainerEngine) NetworkUpdate(ctx context.Context, netName string, opts entities.NetworkUpdateOptions) error {
	options := new(network.UpdateOptions).WithAddDNSServers(opts.AddDNSServers).WithRemoveDNSServers(opts.RemoveDNSServers)
	return network.Update(ic.ClientCtx, netName, options)
}

func (ic *ContainerEngine) NetworkList(ctx context.Context, opts entities.NetworkListOptions) ([]types.Network, error) {
	options := new(network.ListOptions).WithFilters(opts.Filters)
	return network.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) NetworkInspect(ctx context.Context, namesOrIds []string, opts entities.InspectOptions) ([]entities.NetworkInspectReport, []error, error) {
	var (
		reports = make([]entities.NetworkInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	options := new(network.InspectOptions)
	for _, name := range namesOrIds {
		report, err := network.Inspect(ic.ClientCtx, name, options)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("network %s: %w", name, define.ErrNoSuchNetwork))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, report)
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) NetworkReload(ctx context.Context, names []string, opts entities.NetworkReloadOptions) ([]*entities.NetworkReloadReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) NetworkRm(ctx context.Context, namesOrIds []string, opts entities.NetworkRmOptions) ([]*entities.NetworkRmReport, error) {
	reports := make([]*entities.NetworkRmReport, 0, len(namesOrIds))
	options := new(network.RemoveOptions).WithForce(opts.Force)
	if opts.Timeout != nil {
		options = options.WithTimeout(*opts.Timeout)
	}
	for _, name := range namesOrIds {
		response, err := network.Remove(ic.ClientCtx, name, options)
		if err != nil {
			report := &entities.NetworkRmReport{
				Name: name,
				Err:  err,
			}
			reports = append(reports, report)
		} else {
			reports = append(reports, response...)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) NetworkCreate(ctx context.Context, net types.Network, createOptions *types.NetworkCreateOptions) (*types.Network, error) {
	options := new(network.ExtraCreateOptions)
	if createOptions != nil {
		options = options.WithIgnoreIfExists(createOptions.IgnoreIfExists)
	}
	net, err := network.CreateWithOptions(ic.ClientCtx, &net, options)
	if err != nil {
		return nil, err
	}
	return &net, nil
}

// NetworkDisconnect removes a container from a given network
func (ic *ContainerEngine) NetworkDisconnect(ctx context.Context, networkname string, opts entities.NetworkDisconnectOptions) error {
	options := new(network.DisconnectOptions).WithForce(opts.Force)
	return network.Disconnect(ic.ClientCtx, networkname, opts.Container, options)
}

// NetworkConnect removes a container from a given network
func (ic *ContainerEngine) NetworkConnect(ctx context.Context, networkname string, opts entities.NetworkConnectOptions) error {
	return network.Connect(ic.ClientCtx, networkname, opts.Container, &opts.PerNetworkOptions)
}

// NetworkExists checks if the given network exists
func (ic *ContainerEngine) NetworkExists(ctx context.Context, networkname string) (*entities.BoolReport, error) {
	exists, err := network.Exists(ic.ClientCtx, networkname, nil)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{
		Value: exists,
	}, nil
}

// Network prune removes unused networks
func (ic *ContainerEngine) NetworkPrune(ctx context.Context, options entities.NetworkPruneOptions) ([]*entities.NetworkPruneReport, error) {
	opts := new(network.PruneOptions).WithFilters(options.Filters)
	return network.Prune(ic.ClientCtx, opts)
}
