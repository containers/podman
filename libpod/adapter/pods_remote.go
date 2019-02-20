// +build remoteclient

package adapter

import (
	"context"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// Pod ...
type Pod struct {
	remotepod
}

type remotepod struct {
	config  *libpod.PodConfig
	state   *libpod.PodInspectState
	Runtime *LocalRuntime
}

func (r *LocalRuntime) RemovePods(ctx context.Context, cli *cliconfig.PodRmValues) ([]string, []error) {
	var (
		rmErrs []error
		rmPods []string
	)
	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		rmErrs = append(rmErrs, err)
		return nil, rmErrs
	}

	for _, p := range podIDs {
		reply, err := iopodman.RemovePod().Call(r.Conn, p, cli.Force)
		if err != nil {
			rmErrs = append(rmErrs, err)
		} else {
			rmPods = append(rmPods, reply)
		}
	}
	return rmPods, rmErrs
}
