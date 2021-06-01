package abi

import (
	"context"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) NetworkList(ctx context.Context, options entities.NetworkListOptions) ([]*entities.NetworkListReport, error) {
	reports := make([]*entities.NetworkListReport, 0)

	config, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}

	networks, err := network.LoadCNIConfsFromDir(network.GetCNIConfDir(config))
	if err != nil {
		return nil, err
	}

	for _, n := range networks {
		ok, err := network.IfPassesFilter(n, options.Filters)
		if err != nil {
			return nil, err
		}
		if ok {
			reports = append(reports, &entities.NetworkListReport{
				NetworkConfigList: n,
				Labels:            network.GetNetworkLabels(n),
			})
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) NetworkInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]entities.NetworkInspectReport, []error, error) {
	config, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	var errs []error
	rawCNINetworks := make([]entities.NetworkInspectReport, 0, len(namesOrIds))
	for _, name := range namesOrIds {
		rawList, err := network.InspectNetwork(config, name)
		if err != nil {
			if errors.Cause(err) == define.ErrNoSuchNetwork {
				errs = append(errs, errors.Errorf("no such network %s", name))
				continue
			} else {
				return nil, nil, errors.Wrapf(err, "error inspecting network %s", name)
			}
		}
		rawCNINetworks = append(rawCNINetworks, rawList)
	}
	return rawCNINetworks, errs, nil
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
	reports := []*entities.NetworkRmReport{}

	config, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}

	for _, name := range namesOrIds {
		report := entities.NetworkRmReport{Name: name}
		containers, err := ic.Libpod.GetAllContainers()
		if err != nil {
			return reports, err
		}
		// We need to iterate containers looking to see if they belong to the given network
		for _, c := range containers {
			networks, _, err := c.Networks()
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
					if err := ic.Libpod.RemovePod(ctx, pod, true, true); err != nil {
						return reports, err
					}
				} else if err := ic.Libpod.RemoveContainer(ctx, c, true, true); err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
					return reports, err
				}
			}
		}
		if err := network.RemoveNetwork(config, name); err != nil {
			report.Err = err
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) NetworkCreate(ctx context.Context, name string, options entities.NetworkCreateOptions) (*entities.NetworkCreateReport, error) {
	runtimeConfig, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}
	return network.Create(name, options, runtimeConfig)
}

// NetworkDisconnect removes a container from a given network
func (ic *ContainerEngine) NetworkDisconnect(ctx context.Context, networkname string, options entities.NetworkDisconnectOptions) error {
	return ic.Libpod.DisconnectContainerFromNetwork(options.Container, networkname, options.Force)
}

func (ic *ContainerEngine) NetworkConnect(ctx context.Context, networkname string, options entities.NetworkConnectOptions) error {
	return ic.Libpod.ConnectContainerToNetwork(options.Container, networkname, options.Aliases)
}

// NetworkExists checks if the given network exists
func (ic *ContainerEngine) NetworkExists(ctx context.Context, networkname string) (*entities.BoolReport, error) {
	config, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}
	exists, err := network.Exists(config, networkname)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{
		Value: exists,
	}, nil
}

// Network prune removes unused cni networks
func (ic *ContainerEngine) NetworkPrune(ctx context.Context, options entities.NetworkPruneOptions) ([]*entities.NetworkPruneReport, error) {
	runtimeConfig, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	networks, err := network.LoadCNIConfsFromDir(network.GetCNIConfDir(runtimeConfig))
	if err != nil {
		return nil, err
	}

	// Gather up all the non-default networks that the
	// containers want
	networksToKeep := make(map[string]bool)
	for _, c := range cons {
		nets, _, err := c.Networks()
		if err != nil {
			return nil, err
		}
		for _, n := range nets {
			networksToKeep[n] = true
		}
	}
	if len(options.Filters) != 0 {
		for _, n := range networks {
			// This network will be kept anyway
			if _, found := networksToKeep[n.Name]; found {
				continue
			}
			ok, err := network.IfPassesPruneFilter(runtimeConfig, n, options.Filters)
			if err != nil {
				return nil, err
			}
			if !ok {
				networksToKeep[n.Name] = true
			}
		}
	}
	return network.PruneNetworks(runtimeConfig, networksToKeep)
}
