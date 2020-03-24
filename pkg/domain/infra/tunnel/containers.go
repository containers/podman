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

func (ic *ContainerEngine) ContainerPause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		reports []*entities.PauseUnpauseReport
	)
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := containers.Pause(ic.ClientCxt, c.ID)
		reports = append(reports, &entities.PauseUnpauseReport{Id: c.ID, Err: err})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		reports []*entities.PauseUnpauseReport
	)
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := containers.Unpause(ic.ClientCxt, c.ID)
		reports = append(reports, &entities.PauseUnpauseReport{Id: c.ID, Err: err})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	var (
		reports []*entities.StopReport
	)
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		report := entities.StopReport{Id: c.ID}
		report.Err = containers.Stop(ic.ClientCxt, c.ID, &options.Timeout)
		// TODO we need to associate errors returned by http with common
		// define.errors so that we can equity tests. this will allow output
		// to be the same as the native client
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, options entities.KillOptions) ([]*entities.KillReport, error) {
	var (
		reports []*entities.KillReport
	)
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		reports = append(reports, &entities.KillReport{
			Id:  c.ID,
			Err: containers.Kill(ic.ClientCxt, c.ID, options.Signal),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, options entities.RestartOptions) ([]*entities.RestartReport, error) {
	var (
		reports []*entities.RestartReport
		timeout *int
	)
	if options.Timeout != nil {
		t := int(*options.Timeout)
		timeout = &t
	}
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		reports = append(reports, &entities.RestartReport{
			Id:  c.ID,
			Err: containers.Restart(ic.ClientCxt, c.ID, timeout),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, options entities.RmOptions) ([]*entities.RmReport, error) {
	var (
		reports []*entities.RmReport
	)
	ctrs, err := getContainersByContext(ic.ClientCxt, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	// TODO there is no endpoint for container eviction.  Need to discuss
	for _, c := range ctrs {
		reports = append(reports, &entities.RmReport{
			Id:  c.ID,
			Err: containers.Remove(ic.ClientCxt, c.ID, &options.Force, &options.Volumes),
		})
	}
	return reports, nil
}
