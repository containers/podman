// +build ABISupport

package infra

import (
	"context"
	"fmt"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/tunnel"
)

// NewContainerEngine factory provides a libpod runtime for container-related operations
func NewContainerEngine(facts entities.EngineOptions) (entities.ContainerEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), facts.Uri, facts.Identities...)
		return &tunnel.ContainerEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}

// NewContainerEngine factory provides a libpod runtime for image-related operations
func NewImageEngine(facts entities.EngineOptions) (entities.ImageEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodImageRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), facts.Uri, facts.Identities...)
		return &tunnel.ImageEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
