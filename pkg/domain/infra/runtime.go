package infra

import (
	"context"

	"github.com/containers/podman/v6/pkg/bindings"
	"github.com/containers/podman/v6/pkg/domain/entities"
)

// For the meaning of "WithoutLock", compare runtime_tunnel.go:newConnection()
func newConnectionWithoutLock(ctx context.Context, facts *entities.PodmanConfig) (context.Context, error) {
	return bindings.NewConnectionWithOptions(ctx, bindings.Options{
		URI:         facts.URI,
		Identity:    facts.Identity,
		TLSCertFile: facts.TLSCertFile,
		TLSKeyFile:  facts.TLSKeyFile,
		TLSCAFile:   facts.TLSCAFile,
		Machine:     facts.MachineMode,
	})
}
