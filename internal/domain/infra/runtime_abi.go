//go:build !remote

package infra

import (
	"context"
	"fmt"

	ientities "github.com/containers/podman/v5/internal/domain/entities"
	"github.com/containers/podman/v5/internal/domain/infra/tunnel"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

// NewTestingEngine factory provides a libpod runtime for testing-specific operations
func NewTestingEngine(facts *entities.PodmanConfig) (ientities.TestingEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodTestingRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnectionWithIdentity(context.Background(), facts.URI, facts.Identity, facts.MachineMode)
		return &tunnel.TestingEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
