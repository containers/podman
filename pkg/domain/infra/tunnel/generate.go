package tunnel

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/pkg/bindings/generate"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, opts entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	options := new(
		generate.SystemdOptions).
		WithUseName(opts.Name).
		WithContainerPrefix(opts.ContainerPrefix).
		WithNew(opts.New).WithNoHeader(opts.NoHeader).
		WithTemplateUnitFile(opts.TemplateUnitFile).
		WithPodPrefix(opts.PodPrefix).
		WithSeparator(opts.Separator).
		WithWants(opts.Wants).
		WithAfter(opts.After).
		WithRequires(opts.Requires).
		WithAdditionalEnvVariables(opts.AdditionalEnvVariables)

	if opts.StartTimeout != nil {
		options.WithStartTimeout(*opts.StartTimeout)
	}
	if opts.StopTimeout != nil {
		options.WithStopTimeout(*opts.StopTimeout)
	}
	if opts.RestartPolicy != nil {
		options.WithRestartPolicy(*opts.RestartPolicy)
	}
	if opts.RestartSec != nil {
		options.WithRestartSec(*opts.RestartSec)
	}

	return generate.Systemd(ic.ClientCtx, nameOrID, options)
}

// GenerateKube Kubernetes YAML (v1 specification) for nameOrIDs
//
// Note: Caller is responsible for closing returned Reader
func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrIDs []string, opts entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	options := new(generate.KubeOptions).WithService(opts.Service)
	return generate.Kube(ic.ClientCtx, nameOrIDs, options)
}

func (ic *ContainerEngine) GenerateSpec(ctx context.Context, opts *entities.GenerateSpecOptions) (*entities.GenerateSpecReport, error) {
	return nil, fmt.Errorf("GenerateSpec is not supported on the remote API")
}
