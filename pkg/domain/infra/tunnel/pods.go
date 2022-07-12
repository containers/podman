package tunnel

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings/pods"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/util"
)

func (ic *ContainerEngine) PodExists(ctx context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := pods.Exists(ic.ClientCtx, nameOrID, nil)
	return &entities.BoolReport{Value: exists}, err
}

func (ic *ContainerEngine) PodKill(ctx context.Context, namesOrIds []string, opts entities.PodKillOptions) ([]*entities.PodKillReport, error) {
	_, err := util.ParseSignal(opts.Signal)
	if err != nil {
		return nil, err
	}

	foundPods, err := getPodsByContext(ic.ClientCtx, opts.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodKillReport, 0, len(foundPods))
	options := new(pods.KillOptions).WithSignal(opts.Signal)
	for _, p := range foundPods {
		response, err := pods.Kill(ic.ClientCtx, p.Id, options)
		if err != nil {
			report := entities.PodKillReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodLogs(ctx context.Context, nameOrIDs string, options entities.PodLogsOptions) error {
	// PodLogsOptions are similar but contains few extra fields like ctrName
	// So cast other values as is so we can re-use the code
	containerLogsOpts := entities.PodLogsOptionsToContainerLogsOptions(options)

	// interface only accepts slice, keep everything consistent
	name := []string{options.ContainerName}
	return ic.ContainerLogs(ctx, name, containerLogsOpts)
}

func (ic *ContainerEngine) PodPause(ctx context.Context, namesOrIds []string, options entities.PodPauseOptions) ([]*entities.PodPauseReport, error) {
	foundPods, err := getPodsByContext(ic.ClientCtx, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodPauseReport, 0, len(foundPods))
	for _, p := range foundPods {
		response, err := pods.Pause(ic.ClientCtx, p.Id, nil)
		if err != nil {
			report := entities.PodPauseReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodUnpause(ctx context.Context, namesOrIds []string, options entities.PodunpauseOptions) ([]*entities.PodUnpauseReport, error) {
	foundPods, err := getPodsByContext(ic.ClientCtx, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodUnpauseReport, 0, len(foundPods))
	for _, p := range foundPods {
		// If the pod is not paused or degraded, there is no need to attempt an unpause on it
		if p.Status != define.PodStatePaused && p.Status != define.PodStateDegraded {
			continue
		}
		response, err := pods.Unpause(ic.ClientCtx, p.Id, nil)
		if err != nil {
			report := entities.PodUnpauseReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStop(ctx context.Context, namesOrIds []string, opts entities.PodStopOptions) ([]*entities.PodStopReport, error) {
	timeout := -1
	foundPods, err := getPodsByContext(ic.ClientCtx, opts.All, namesOrIds)
	if err != nil && !(opts.Ignore && errors.Is(err, define.ErrNoSuchPod)) {
		return nil, err
	}
	if opts.Timeout != -1 {
		timeout = opts.Timeout
	}
	reports := make([]*entities.PodStopReport, 0, len(foundPods))
	options := new(pods.StopOptions).WithTimeout(timeout)
	for _, p := range foundPods {
		response, err := pods.Stop(ic.ClientCtx, p.Id, options)
		if err != nil {
			report := entities.PodStopReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRestart(ctx context.Context, namesOrIds []string, options entities.PodRestartOptions) ([]*entities.PodRestartReport, error) {
	foundPods, err := getPodsByContext(ic.ClientCtx, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodRestartReport, 0, len(foundPods))
	for _, p := range foundPods {
		response, err := pods.Restart(ic.ClientCtx, p.Id, nil)
		if err != nil {
			report := entities.PodRestartReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStart(ctx context.Context, namesOrIds []string, options entities.PodStartOptions) ([]*entities.PodStartReport, error) {
	foundPods, err := getPodsByContext(ic.ClientCtx, options.All, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodStartReport, 0, len(foundPods))
	for _, p := range foundPods {
		response, err := pods.Start(ic.ClientCtx, p.Id, nil)
		if err != nil {
			report := entities.PodStartReport{
				Errs: []error{err},
				Id:   p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRm(ctx context.Context, namesOrIds []string, opts entities.PodRmOptions) ([]*entities.PodRmReport, error) {
	foundPods, err := getPodsByContext(ic.ClientCtx, opts.All, namesOrIds)
	if err != nil && !(opts.Ignore && errors.Is(err, define.ErrNoSuchPod)) {
		return nil, err
	}
	reports := make([]*entities.PodRmReport, 0, len(foundPods))
	options := new(pods.RemoveOptions).WithForce(opts.Force)
	if opts.Timeout != nil {
		options = options.WithTimeout(*opts.Timeout)
	}
	for _, p := range foundPods {
		response, err := pods.Remove(ic.ClientCtx, p.Id, options)
		if err != nil {
			report := entities.PodRmReport{
				Err: err,
				Id:  p.Id,
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, response)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodPrune(ctx context.Context, opts entities.PodPruneOptions) ([]*entities.PodPruneReport, error) {
	return pods.Prune(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) PodCreate(ctx context.Context, specg entities.PodSpec) (*entities.PodCreateReport, error) {
	return pods.CreatePodFromSpec(ic.ClientCtx, &specg)
}

func (ic *ContainerEngine) PodClone(ctx context.Context, podClone entities.PodCloneOptions) (*entities.PodCloneReport, error) {
	return nil, nil
}

func (ic *ContainerEngine) PodTop(ctx context.Context, opts entities.PodTopOptions) (*entities.StringSliceReport, error) {
	switch {
	case opts.Latest:
		return nil, errors.New("latest is not supported")
	case opts.NameOrID == "":
		return nil, errors.New("NameOrID must be specified")
	}
	options := new(pods.TopOptions).WithDescriptors(opts.Descriptors)
	topOutput, err := pods.Top(ic.ClientCtx, opts.NameOrID, options)
	if err != nil {
		return nil, err
	}
	return &entities.StringSliceReport{Value: topOutput}, nil
}

func (ic *ContainerEngine) PodPs(ctx context.Context, opts entities.PodPSOptions) ([]*entities.ListPodsReport, error) {
	options := new(pods.ListOptions).WithFilters(opts.Filters)
	return pods.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) PodInspect(ctx context.Context, options entities.PodInspectOptions) (*entities.PodInspectReport, error) {
	switch {
	case options.Latest:
		return nil, errors.New("latest is not supported")
	case options.NameOrID == "":
		return nil, errors.New("NameOrID must be specified")
	}
	return pods.Inspect(ic.ClientCtx, options.NameOrID, nil)
}

func (ic *ContainerEngine) PodStats(ctx context.Context, namesOrIds []string, opts entities.PodStatsOptions) ([]*entities.PodStatsReport, error) {
	options := new(pods.StatsOptions).WithAll(opts.All)
	return pods.Stats(ic.ClientCtx, namesOrIds, options)
}
