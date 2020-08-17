// +build !remote

package infra

import (
	"context"

	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	flag "github.com/spf13/pflag"
)

// ContainerEngine Proxy will be EOL'ed after podman is separated from libpod repo

func NewLibpodRuntime(flags *flag.FlagSet, opts *entities.PodmanConfig) (entities.ContainerEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ContainerEngine{Libpod: r}, nil
}

func NewLibpodImageRuntime(flags *flag.FlagSet, opts *entities.PodmanConfig) (entities.ImageEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ImageEngine{Libpod: r}, nil
}
