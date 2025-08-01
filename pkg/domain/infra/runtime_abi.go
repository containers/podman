//go:build !remote

package infra

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/tunnel"
)

// NewContainerEngine factory provides a libpod runtime for container-related operations
func NewContainerEngine(facts *entities.PodmanConfig) (entities.ContainerEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnectionWithOptions(context.Background(), bindings.Options{
			URI:         facts.URI,
			Identity:    facts.Identity,
			TLSCertFile: facts.TLSCertFile,
			TLSKeyFile:  facts.TLSKeyFile,
			TLSCAFile:   facts.TLSCAFile,
			Machine:     facts.MachineMode,
		})
		return &tunnel.ContainerEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}

// NewImageEngine factory provides a libpod runtime for image-related operations
func NewImageEngine(facts *entities.PodmanConfig) (entities.ImageEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodImageRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		// TODO: look at me!
		ctx, err := bindings.NewConnectionWithOptions(context.Background(), bindings.Options{
			URI:         facts.URI,
			Identity:    facts.Identity,
			TLSCertFile: facts.TLSCertFile,
			TLSKeyFile:  facts.TLSKeyFile,
			TLSCAFile:   facts.TLSCAFile,
			Machine:     facts.MachineMode,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, facts.URI)
		}
		return &tunnel.ImageEngine{ClientCtx: ctx, FarmNode: tunnel.FarmNode{NodeName: facts.FarmNodeName}}, nil
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
