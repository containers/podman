//go:build !remote

package infra

import (
	"context"

	ientities "github.com/containers/podman/v6/internal/domain/entities"
	"github.com/containers/podman/v6/internal/domain/infra/abi"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/domain/infra"
	flag "github.com/spf13/pflag"
	"go.podman.io/storage"
)

func NewLibpodTestingRuntime(flags *flag.FlagSet, opts *entities.PodmanConfig) (ientities.TestingEngine, error) {
	r, err := infra.GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	store, err := storage.GetStore(r.StorageConfig())
	if err != nil {
		return nil, err
	}
	return &abi.TestingEngine{Libpod: r, Store: store}, nil
}
