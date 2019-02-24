// +build !remoteclient

package adapter

import (
	"context"
	"github.com/containers/libpod/pkg/adapter/shortcuts"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
)

// Pod ...
type Pod struct {
	*libpod.Pod
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
		if err := r.RemovePod(ctx, p, cli.Force, cli.Force); err != nil {
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
