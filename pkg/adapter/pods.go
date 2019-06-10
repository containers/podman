// +build !remoteclient

package adapter

import (
	"context"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PodContainerStats is struct containing an adapter Pod and a libpod
// ContainerStats and is used primarily for outputing pod stats.
type PodContainerStats struct {
	Pod            *Pod
	ContainerStats map[string]*libpod.ContainerStats
}

// PrunePods removes pods
func (r *LocalRuntime) PrunePods(ctx context.Context, cli *cliconfig.PodPruneValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	maxWorkers := shared.DefaultPoolSize("rm")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	states := []string{shared.PodStateStopped, shared.PodStateExited}
	if cli.Force {
		states = append(states, shared.PodStateRunning)
	}

	pods, err := r.GetPodsByStatus(states)
	if err != nil {
		return ok, failures, err
	}
	if len(pods) < 1 {
		return ok, failures, nil
	}

	pool := shared.NewPool("pod_prune", maxWorkers, len(pods))
	for _, p := range pods {
		p := p

		pool.Add(shared.Job{p.ID(),
			func() error {
				err := r.Runtime.RemovePod(ctx, p, cli.Force, cli.Force)
				if err != nil {
					logrus.Debugf("Failed to remove pod %s: %s", p.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// RemovePods ...
func (r *LocalRuntime) RemovePods(ctx context.Context, cli *cliconfig.PodRmValues) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	for _, p := range pods {
		if err := r.Runtime.RemovePod(ctx, p, cli.Force, cli.Force); err != nil {
			errs = append(errs, err)
		} else {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// GetLatestPod gets the latest pod and wraps it in an adapter pod
func (r *LocalRuntime) GetLatestPod() (*Pod, error) {
	pod := Pod{}
	p, err := r.Runtime.GetLatestPod()
	pod.Pod = p
	return &pod, err
}

// GetAllPods gets all pods and wraps it in an adapter pod
func (r *LocalRuntime) GetAllPods() ([]*Pod, error) {
	var pods []*Pod
	allPods, err := r.Runtime.GetAllPods()
	if err != nil {
		return nil, err
	}
	for _, p := range allPods {
		pod := Pod{}
		pod.Pod = p
		pods = append(pods, &pod)
	}
	return pods, nil
}

// LookupPod gets a pod by name or id and wraps it in an adapter pod
func (r *LocalRuntime) LookupPod(nameOrID string) (*Pod, error) {
	pod := Pod{}
	p, err := r.Runtime.LookupPod(nameOrID)
	pod.Pod = p
	return &pod, err
}

// StopPods is a wrapper to libpod to stop pods based on a cli context
func (r *LocalRuntime) StopPods(ctx context.Context, cli *cliconfig.PodStopValues) ([]string, []error) {
	timeout := -1
	if cli.Flags().Changed("timeout") {
		timeout = int(cli.Timeout)
	}
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	for _, p := range pods {
		stopped := true
		conErrs, stopErr := p.StopWithTimeout(ctx, true, int(timeout))
		if stopErr != nil {
			errs = append(errs, stopErr)
			stopped = false
		}
		if conErrs != nil {
			stopped = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if stopped {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// KillPods is a wrapper to libpod to start pods based on the cli context
func (r *LocalRuntime) KillPods(ctx context.Context, cli *cliconfig.PodKillValues, signal uint) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}
	for _, p := range pods {
		killed := true
		conErrs, killErr := p.Kill(signal)
		if killErr != nil {
			errs = append(errs, killErr)
			killed = false
		}
		if conErrs != nil {
			killed = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if killed {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// StartPods is a wrapper to start pods based on the cli context
func (r *LocalRuntime) StartPods(ctx context.Context, cli *cliconfig.PodStartValues) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}
	for _, p := range pods {
		started := true
		conErrs, startErr := p.Start(ctx)
		if startErr != nil {
			errs = append(errs, startErr)
			started = false
		}
		if conErrs != nil {
			started = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if started {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// CreatePod is a wrapper for libpod and creating a new pod from the cli context
func (r *LocalRuntime) CreatePod(ctx context.Context, cli *cliconfig.PodCreateValues, labels map[string]string) (string, error) {
	var (
		options []libpod.PodCreateOption
		err     error
	)

	if cli.Flag("cgroup-parent").Changed {
		options = append(options, libpod.WithPodCgroupParent(cli.CgroupParent))
	}

	if len(labels) != 0 {
		options = append(options, libpod.WithPodLabels(labels))
	}

	if cli.Flag("name").Changed {
		options = append(options, libpod.WithPodName(cli.Name))
	}

	if cli.Infra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := shared.GetNamespaceOptions(strings.Split(cli.Share, ","))
		if err != nil {
			return "", err
		}
		options = append(options, nsOptions...)
	}

	if len(cli.Publish) > 0 {
		portBindings, err := shared.CreatePortBindings(cli.Publish)
		if err != nil {
			return "", err
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	// always have containers use pod cgroups
	// User Opt out is not yet supported
	options = append(options, libpod.WithPodCgroups())

	pod, err := r.NewPod(ctx, options...)
	if err != nil {
		return "", err
	}
	return pod.ID(), nil
}

// GetPodStatus is a wrapper to get the status of a local libpod pod
func (p *Pod) GetPodStatus() (string, error) {
	return shared.GetPodStatus(p.Pod)
}

// BatchContainerOp is a wrapper for the shared function of the same name
func BatchContainerOp(ctr *libpod.Container, opts shared.PsOptions) (shared.BatchContainerStruct, error) {
	return shared.BatchContainerOp(ctr, opts)
}

// PausePods is a wrapper for pausing pods via libpod
func (r *LocalRuntime) PausePods(c *cliconfig.PodPauseValues) ([]string, map[string]error, []error) {
	var (
		pauseIDs    []string
		pauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		pauseErrors = append(pauseErrors, err)
		return nil, containerErrors, pauseErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Pause()
		if err != nil {
			pauseErrors = append(pauseErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		pauseIDs = append(pauseIDs, pod.ID())

	}
	return pauseIDs, containerErrors, pauseErrors
}

// UnpausePods is a wrapper for unpausing pods via libpod
func (r *LocalRuntime) UnpausePods(c *cliconfig.PodUnpauseValues) ([]string, map[string]error, []error) {
	var (
		unpauseIDs    []string
		unpauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		unpauseErrors = append(unpauseErrors, err)
		return nil, containerErrors, unpauseErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Unpause()
		if err != nil {
			unpauseErrors = append(unpauseErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		unpauseIDs = append(unpauseIDs, pod.ID())

	}
	return unpauseIDs, containerErrors, unpauseErrors
}

// RestartPods is a wrapper to restart pods via libpod
func (r *LocalRuntime) RestartPods(ctx context.Context, c *cliconfig.PodRestartValues) ([]string, map[string]error, []error) {
	var (
		restartIDs    []string
		restartErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		restartErrors = append(restartErrors, err)
		return nil, containerErrors, restartErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Restart(ctx)
		if err != nil {
			restartErrors = append(restartErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		restartIDs = append(restartIDs, pod.ID())

	}
	return restartIDs, containerErrors, restartErrors

}

// PodTop is a wrapper function to call GetPodPidInformation in libpod and return its results
// for output
func (r *LocalRuntime) PodTop(c *cliconfig.PodTopValues, descriptors []string) ([]string, error) {
	var (
		pod *Pod
		err error
	)

	if c.Latest {
		pod, err = r.GetLatestPod()
	} else {
		pod, err = r.LookupPod(c.InputArgs[0])
	}
	if err != nil {
		return nil, errors.Wrapf(err, "unable to lookup requested container")
	}
	podStatus, err := pod.GetPodStatus()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get status for pod %s", pod.ID())
	}
	if podStatus != "Running" {
		return nil, errors.Errorf("pod top can only be used on pods with at least one running container")
	}
	return pod.GetPodPidInformation(descriptors)
}

// GetStatPods returns pods for use in pod stats
func (r *LocalRuntime) GetStatPods(c *cliconfig.PodStatsValues) ([]*Pod, error) {
	var (
		adapterPods []*Pod
		pods        []*libpod.Pod
		err         error
	)

	if len(c.InputArgs) > 0 || c.Latest || c.All {
		pods, err = shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	} else {
		pods, err = r.Runtime.GetRunningPods()
	}
	if err != nil {
		return nil, err
	}
	// convert libpod pods to adapter pods
	for _, p := range pods {
		adapterPod := Pod{
			p,
		}
		adapterPods = append(adapterPods, &adapterPod)
	}
	return adapterPods, nil
}
