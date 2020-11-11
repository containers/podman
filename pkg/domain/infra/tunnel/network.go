package tunnel

import (
	"context"

	"github.com/containers/podman/v2/pkg/bindings/network"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) NetworkList(ctx context.Context, options entities.NetworkListOptions) ([]*entities.NetworkListReport, error) {
	return network.List(ic.ClientCxt, options)
}

func (ic *ContainerEngine) NetworkInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]entities.NetworkInspectReport, []error, error) {
	var (
		reports = make([]entities.NetworkInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	for _, name := range namesOrIds {
		report, err := network.Inspect(ic.ClientCxt, name)
		if err != nil {
			errModel, ok := err.(entities.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, errors.Errorf("no such network %q", name))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, report...)
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) NetworkRm(ctx context.Context, namesOrIds []string, options entities.NetworkRmOptions) ([]*entities.NetworkRmReport, error) {
	reports := make([]*entities.NetworkRmReport, 0, len(namesOrIds))
	for _, name := range namesOrIds {
		response, err := network.Remove(ic.ClientCxt, name, &options.Force)
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

func (ic *ContainerEngine) NetworkCreate(ctx context.Context, name string, options entities.NetworkCreateOptions) (*entities.NetworkCreateReport, error) {
	return network.Create(ic.ClientCxt, options, &name)
}

// NetworkDisconnect removes a container from a given network
func (ic *ContainerEngine) NetworkDisconnect(ctx context.Context, networkname string, options entities.NetworkDisconnectOptions) error {
	return network.Disconnect(ic.ClientCxt, networkname, options)
}

// NetworkConnect removes a container from a given network
func (ic *ContainerEngine) NetworkConnect(ctx context.Context, networkname string, options entities.NetworkConnectOptions) error {
	return network.Connect(ic.ClientCxt, networkname, options)
}
