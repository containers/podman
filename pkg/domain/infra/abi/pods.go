package abi

import (
	"context"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	lpfilters "github.com/containers/podman/v2/libpod/filters"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/containers/podman/v2/pkg/specgen/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getPodsByContext returns a slice of pods. Note that all, latest and pods are
// mutually exclusive arguments.
func getPodsByContext(all, latest bool, pods []string, runtime *libpod.Runtime) ([]*libpod.Pod, error) {
	var outpods []*libpod.Pod
	if all {
		return runtime.GetAllPods()
	}
	if latest {
		p, err := runtime.GetLatestPod()
		if err != nil {
			return nil, err
		}
		outpods = append(outpods, p)
		return outpods, nil
	}
	var err error
	for _, p := range pods {
		pod, e := runtime.LookupPod(p)
		if e != nil {
			// Log all errors here, so callers don't need to.
			logrus.Debugf("Error looking up pod %q: %v", p, e)
			if err == nil {
				err = e
			}
		} else {
			outpods = append(outpods, pod)
		}
	}
	return outpods, err
}

func (ic *ContainerEngine) PodExists(ctx context.Context, nameOrID string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupPod(nameOrID)
	if err != nil && errors.Cause(err) != define.ErrNoSuchPod {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) PodKill(ctx context.Context, namesOrIds []string, options entities.PodKillOptions) ([]*entities.PodKillReport, error) {
	reports := []*entities.PodKillReport{}
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		report := entities.PodKillReport{Id: p.ID()}
		conErrs, err := p.Kill(ctx, uint(sig))
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(conErrs) > 0 {
			for id, err := range conErrs {
				report.Errs = append(report.Errs, errors.Wrapf(err, "error killing container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodPause(ctx context.Context, namesOrIds []string, options entities.PodPauseOptions) ([]*entities.PodPauseReport, error) {
	reports := []*entities.PodPauseReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodPauseReport{Id: p.ID()}
		errs, err := p.Pause(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, errors.Wrapf(v, "error pausing container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodUnpause(ctx context.Context, namesOrIds []string, options entities.PodunpauseOptions) ([]*entities.PodUnpauseReport, error) {
	reports := []*entities.PodUnpauseReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodUnpauseReport{Id: p.ID()}
		errs, err := p.Unpause(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, errors.Wrapf(v, "error unpausing container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStop(ctx context.Context, namesOrIds []string, options entities.PodStopOptions) ([]*entities.PodStopReport, error) {
	reports := []*entities.PodStopReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchPod) {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodStopReport{Id: p.ID()}
		errs, err := p.StopWithTimeout(ctx, false, options.Timeout)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, errors.Wrapf(v, "error stopping container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRestart(ctx context.Context, namesOrIds []string, options entities.PodRestartOptions) ([]*entities.PodRestartReport, error) {
	reports := []*entities.PodRestartReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodRestartReport{Id: p.ID()}
		errs, err := p.Restart(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, errors.Wrapf(v, "error restarting container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStart(ctx context.Context, namesOrIds []string, options entities.PodStartOptions) ([]*entities.PodStartReport, error) {
	reports := []*entities.PodStartReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		report := entities.PodStartReport{Id: p.ID()}
		errs, err := p.Start(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, errors.Wrapf(v, "error starting container %s", id))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRm(ctx context.Context, namesOrIds []string, options entities.PodRmOptions) ([]*entities.PodRmReport, error) {
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchPod) {
		return nil, err
	}
	reports := make([]*entities.PodRmReport, 0, len(pods))
	for _, p := range pods {
		report := entities.PodRmReport{Id: p.ID()}
		err := ic.Libpod.RemovePod(ctx, p, true, options.Force)
		if err != nil {
			report.Err = err
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodPrune(ctx context.Context, options entities.PodPruneOptions) ([]*entities.PodPruneReport, error) {
	return ic.prunePodHelper(ctx)
}

func (ic *ContainerEngine) prunePodHelper(ctx context.Context) ([]*entities.PodPruneReport, error) {
	response, err := ic.Libpod.PrunePods(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodPruneReport, 0, len(response))
	for k, v := range response {
		reports = append(reports, &entities.PodPruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) PodCreate(ctx context.Context, opts entities.PodCreateOptions) (*entities.PodCreateReport, error) {
	podSpec := specgen.NewPodSpecGenerator()
	opts.ToPodSpecGen(podSpec)
	pod, err := generate.MakePod(podSpec, ic.Libpod)
	if err != nil {
		return nil, err
	}
	return &entities.PodCreateReport{Id: pod.ID()}, nil
}

func (ic *ContainerEngine) PodTop(ctx context.Context, options entities.PodTopOptions) (*entities.StringSliceReport, error) {
	var (
		pod *libpod.Pod
		err error
	)

	// Look up the pod.
	if options.Latest {
		pod, err = ic.Libpod.GetLatestPod()
	} else {
		pod, err = ic.Libpod.LookupPod(options.NameOrID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to lookup requested container")
	}

	// Run Top.
	report := &entities.StringSliceReport{}
	report.Value, err = pod.GetPodPidInformation(options.Descriptors)
	return report, err
}

func (ic *ContainerEngine) PodPs(ctx context.Context, options entities.PodPSOptions) ([]*entities.ListPodsReport, error) {
	var (
		err error
		pds = []*libpod.Pod{}
	)

	filters := make([]libpod.PodFilter, 0, len(options.Filters))
	for k, v := range options.Filters {
		f, err := lpfilters.GeneratePodFilterFunc(k, v)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	if options.Latest {
		pod, err := ic.Libpod.GetLatestPod()
		if err != nil {
			return nil, err
		}
		pds = append(pds, pod)
	} else {
		pds, err = ic.Libpod.Pods(filters...)
		if err != nil {
			return nil, err
		}
	}

	reports := make([]*entities.ListPodsReport, 0, len(pds))
	for _, p := range pds {
		var lpcs []*entities.ListPodContainer
		status, err := p.GetPodStatus()
		if err != nil {
			return nil, err
		}
		cons, err := p.AllContainers()
		if err != nil {
			return nil, err
		}
		for _, c := range cons {
			state, err := c.State()
			if err != nil {
				return nil, err
			}
			lpcs = append(lpcs, &entities.ListPodContainer{
				Id:     c.ID(),
				Names:  c.Name(),
				Status: state.String(),
			})
		}
		infraID, err := p.InfraContainerID()
		if err != nil {
			return nil, err
		}
		reports = append(reports, &entities.ListPodsReport{
			Cgroup:     p.CgroupParent(),
			Containers: lpcs,
			Created:    p.CreatedTime(),
			Id:         p.ID(),
			InfraId:    infraID,
			Name:       p.Name(),
			Namespace:  p.Namespace(),
			Status:     status,
			Labels:     p.Labels(),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) PodInspect(ctx context.Context, options entities.PodInspectOptions) (*entities.PodInspectReport, error) {
	var (
		pod *libpod.Pod
		err error
	)
	// Look up the pod.
	if options.Latest {
		pod, err = ic.Libpod.GetLatestPod()
	} else {
		pod, err = ic.Libpod.LookupPod(options.NameOrID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to lookup requested container")
	}
	inspect, err := pod.Inspect()
	if err != nil {
		return nil, err
	}
	return &entities.PodInspectReport{InspectPodData: inspect}, nil
}
