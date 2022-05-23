package abi

import (
	"context"

	"github.com/containers/common/libnetwork/types"
	netutil "github.com/containers/common/libnetwork/util"
	"github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) NetworkList(ctx context.Context, options entities.NetworkListOptions) ([]types.Network, error) {
	filters, err := netutil.GenerateNetworkFilters(options.Filters)
	if err != nil {
		return nil, err
	}
	nets, err := ic.Libpod.Network().NetworkList(filters...)
	return nets, err
}

func (ic *ContainerEngine) NetworkInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]types.Network, []error, error) {
	var errs []error
	networks := make([]types.Network, 0, len(namesOrIds))
	for _, name := range namesOrIds {
		net, err := ic.Libpod.Network().NetworkInspect(name)
		if err != nil {
			if errors.Cause(err) == define.ErrNoSuchNetwork {
				errs = append(errs, errors.Wrapf(err, "network %s", name))
				continue
			} else {
				return nil, nil, errors.Wrapf(err, "error inspecting network %s", name)
			}
		}
		networks = append(networks, net)
	}
	return networks, errs, nil
}

func (ic *ContainerEngine) NetworkReload(ctx context.Context, names []string, options entities.NetworkReloadOptions) ([]*entities.NetworkReloadReport, error) {
	ctrs, err := getContainersByContext(options.All, options.Latest, names, ic.Libpod)
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.NetworkReloadReport, 0, len(ctrs))
	for _, ctr := range ctrs {
		report := new(entities.NetworkReloadReport)
		report.Id = ctr.ID()
		report.Err = ctr.ReloadNetwork()
		// ignore errors for invalid ctr state and network mode when --all is used
		if options.All && (errors.Cause(report.Err) == define.ErrCtrStateInvalid ||
			errors.Cause(report.Err) == define.ErrNetworkModeInvalid) {
			continue
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func (ic *ContainerEngine) NetworkRm(ctx context.Context, namesOrIds []string, options entities.NetworkRmOptions) ([]*entities.NetworkRmReport, error) {
	reports := make([]*entities.NetworkRmReport, 0, len(namesOrIds))

	for _, name := range namesOrIds {
		report := entities.NetworkRmReport{Name: name}
		containers, err := ic.Libpod.GetAllContainers()
		if err != nil {
			return reports, err
		}
		// We need to iterate containers looking to see if they belong to the given network
		for _, c := range containers {
			networks, err := c.Networks()
			// if container vanished or network does not exist, go to next container
			if errors.Is(err, define.ErrNoSuchNetwork) || errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			if err != nil {
				return reports, err
			}
			if util.StringInSlice(name, networks) {
				// if user passes force, we nuke containers and pods
				if !options.Force {
					// Without the force option, we return an error
					return reports, errors.Wrapf(define.ErrNetworkInUse, "%q has associated containers with it. Use -f to forcibly delete containers and pods", name)
				}
				if c.IsInfra() {
					// if we have a infra container we need to remove the pod
					pod, err := ic.Libpod.GetPod(c.PodID())
					if err != nil {
						return reports, err
					}
					if err := ic.Libpod.RemovePod(ctx, pod, true, true, options.Timeout); err != nil {
						return reports, err
					}
				} else if err := ic.Libpod.RemoveContainer(ctx, c, true, true, options.Timeout); err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
					return reports, err
				}
			}
		}
		if err := ic.Libpod.Network().NetworkRemove(name); err != nil {
			report.Err = err
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) NetworkCreate(ctx context.Context, network types.Network) (*types.Network, error) {
	if util.StringInSlice(network.Name, []string{"none", "host", "bridge", "private", "slirp4netns", "container", "ns"}) {
		return nil, errors.Errorf("cannot create network with name %q because it conflicts with a valid network mode", network.Name)
	}
	network, err := ic.Libpod.Network().NetworkCreate(network)
	if err != nil {
		return nil, err
	}
	return &network, nil
}

// NetworkDisconnect removes a container from a given network
func (ic *ContainerEngine) NetworkDisconnect(ctx context.Context, networkname string, options entities.NetworkDisconnectOptions) error {
	return ic.Libpod.DisconnectContainerFromNetwork(options.Container, networkname, options.Force)
}

func (ic *ContainerEngine) NetworkConnect(ctx context.Context, networkname string, options entities.NetworkConnectOptions) error {
	return ic.Libpod.ConnectContainerToNetwork(options.Container, networkname, options.PerNetworkOptions)
}

// NetworkExists checks if the given network exists
func (ic *ContainerEngine) NetworkExists(ctx context.Context, networkname string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.Network().NetworkInspect(networkname)
	exists := true
	// if err is ErrNoSuchNetwork do not return it
	if errors.Is(err, define.ErrNoSuchNetwork) {
		exists = false
	} else if err != nil {
		return nil, err
	}
	return &entities.BoolReport{
		Value: exists,
	}, nil
}

// Network prune removes unused cni networks
func (ic *ContainerEngine) NetworkPrune(ctx context.Context, options entities.NetworkPruneOptions) ([]*entities.NetworkPruneReport, error) {
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	// Gather up all the non-default networks that the
	// containers want
	networksToKeep := make(map[string]bool)
	for _, c := range cons {
		nets, err := c.Networks()
		if err != nil {
			return nil, err
		}
		for _, n := range nets {
			networksToKeep[n] = true
		}
	}
	// ignore the default network, this one cannot be deleted
	networksToKeep[ic.Libpod.GetDefaultNetworkName()] = true

	// get all filters
	filters, err := netutil.GenerateNetworkPruneFilters(options.Filters)
	if err != nil {
		return nil, err
	}
	danglingFilterFunc := func(net types.Network) bool {
		for network := range networksToKeep {
			if network == net.Name {
				return false
			}
		}
		return true
	}
	filters = append(filters, danglingFilterFunc)
	nets, err := ic.Libpod.Network().NetworkList(filters...)
	if err != nil {
		return nil, err
	}

	pruneReport := make([]*entities.NetworkPruneReport, 0, len(nets))
	for _, net := range nets {
		pruneReport = append(pruneReport, &entities.NetworkPruneReport{
			Name:  net.Name,
			Error: ic.Libpod.Network().NetworkRemove(net.Name),
		})
	}
	return pruneReport, nil
}
