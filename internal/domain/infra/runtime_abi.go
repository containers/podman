//go:build !remote

package infra

import (
	"context"
	"fmt"

	ientities "github.com/containers/podman/v6/internal/domain/entities"
	"github.com/containers/podman/v6/internal/domain/infra/tunnel"
	"github.com/containers/podman/v6/pkg/bindings"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"go.podman.io/image/v5/pkg/cli/basetls/tlsdetails"
)

// NewTestingEngine factory provides a libpod runtime for testing-specific operations
func NewTestingEngine(facts *entities.PodmanConfig) (ientities.TestingEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodTestingRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		baseTLSConfig, err := tlsdetails.BaseTLSFromOptionalFile(facts.TLSDetailsFile)
		if err != nil {
			return nil, err
		}
		ctx, err := bindings.NewConnectionWithOptions(context.Background(), bindings.Options{
			URI:           facts.URI,
			Identity:      facts.Identity,
			TLSCertFile:   facts.TLSCertFile,
			TLSKeyFile:    facts.TLSKeyFile,
			TLSCAFile:     facts.TLSCAFile,
			BaseTLSConfig: baseTLSConfig.TLSConfig(),
			Machine:       facts.MachineMode,
		})
		return &tunnel.TestingEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
