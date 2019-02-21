// +build !remoteclient

package adapter

import (
	"context"
	"github.com/containers/libpod/libpod/adapter/shortcuts"

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
