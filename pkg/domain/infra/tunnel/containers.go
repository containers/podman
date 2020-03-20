package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	exists, err := containers.Exists(ic.ClientCxt, nameOrId)
	return &entities.BoolReport{Value: exists}, err
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	var (
		responses []entities.WaitReport
	)
	cons, err := getContainersByContext(ic.ClientCxt, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range cons {
		response := entities.WaitReport{Id: c.ID}
		exitCode, err := containers.Wait(ic.ClientCxt, c.ID, &options.Condition)
		if err != nil {
			response.Error = err
		} else {
			response.ExitCode = exitCode
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (r *ContainerEngine) ContainerDelete(ctx context.Context, opts entities.ContainerDeleteOptions) (*entities.ContainerDeleteReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) ContainerPrune(ctx context.Context) (*entities.ContainerPruneReport, error) {
	panic("implement me")
}
