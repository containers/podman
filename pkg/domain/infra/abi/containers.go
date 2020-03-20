// +build ABISupport

package abi

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

// TODO: Should return *entities.ContainerExistsReport, error
func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupContainer(nameOrId)
	if err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	var (
		responses []entities.WaitReport
	)
	condition, err := define.StringToContainerStatus(options.Condition)
	if err != nil {
		return nil, err
	}

	ctrs, err := shortcuts.GetContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		response := entities.WaitReport{Id: c.ID()}
		exitCode, err := c.WaitForConditionWithInterval(options.Interval, condition)
		if err != nil {
			response.Error = err
		} else {
			response.ExitCode = exitCode
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (ic *ContainerEngine) ContainerDelete(ctx context.Context, opts entities.ContainerDeleteOptions) (*entities.ContainerDeleteReport, error) {
	panic("implement me")
}

func (ic *ContainerEngine) ContainerPrune(ctx context.Context) (*entities.ContainerPruneReport, error) {
	panic("implement me")
}

func (ic *ContainerEngine) PodDelete(ctx context.Context, opts entities.PodPruneOptions) (*entities.PodDeleteReport, error) {
	panic("implement me")
}

func (ic *ContainerEngine) PodPrune(ctx context.Context) (*entities.PodPruneReport, error) {
	panic("implement me")
}

func (ic *ContainerEngine) VolumeDelete(ctx context.Context, opts entities.VolumeDeleteOptions) (*entities.VolumeDeleteReport, error) {
	panic("implement me")
}

func (ic *ContainerEngine) VolumePrune(ctx context.Context) (*entities.VolumePruneReport, error) {
	panic("implement me")
}
