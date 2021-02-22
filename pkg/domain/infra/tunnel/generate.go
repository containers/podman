package tunnel

import (
	"context"

	"github.com/containers/podman/v3/pkg/bindings/generate"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, opts entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	options := new(generate.SystemdOptions).WithUseName(opts.Name).WithContainerPrefix(opts.ContainerPrefix).WithNew(opts.New).WithNoHeader(opts.NoHeader)
	options.WithPodPrefix(opts.PodPrefix).WithRestartPolicy(opts.RestartPolicy).WithSeparator(opts.Separator)
	if to := opts.StopTimeout; to != nil {
		options.WithStopTimeout(*opts.StopTimeout)
	}
	return generate.Systemd(ic.ClientCtx, nameOrID, options)
}

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrIDs []string, opts entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	options := new(generate.KubeOptions).WithService(opts.Service)
	return generate.Kube(ic.ClientCtx, nameOrIDs, options)
}
