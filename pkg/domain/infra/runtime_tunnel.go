// +build !ABISupport

package infra

import (
	"context"
	"fmt"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/tunnel"
)

func NewContainerEngine(opts entities.EngineOptions) (entities.ContainerEngine, error) {
	switch opts.EngineMode {
	case entities.ABIMode:
		return nil, fmt.Errorf("direct runtime not supported")
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), opts.Uri, opts.Identities...)
		return &tunnel.ContainerEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", opts.EngineMode)
}

// NewImageEngine factory provides a libpod runtime for image-related operations
func NewImageEngine(opts entities.EngineOptions) (entities.ImageEngine, error) {
	switch opts.EngineMode {
	case entities.ABIMode:
		return nil, fmt.Errorf("direct image runtime not supported")
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), opts.Uri, opts.Identities...)
		return &tunnel.ImageEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", opts.EngineMode)
}
