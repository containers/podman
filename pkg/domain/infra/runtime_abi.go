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
func NewContainerEngine(opts entities.EngineOptions) (entities.ContainerEngine, error) {
	switch opts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodRuntime(opts.FlagSet, opts.Flags)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), opts.Uri, opts.Identities...)
		return &tunnel.ContainerEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", opts.EngineMode)
}

// NewContainerEngine factory provides a libpod runtime for image-related operations
func NewImageEngine(opts entities.EngineOptions) (entities.ImageEngine, error) {
	switch opts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodImageRuntime(opts.FlagSet, opts.Flags)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnection(context.Background(), opts.Uri, opts.Identities...)
		return &tunnel.ImageEngine{ClientCxt: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", opts.EngineMode)
}
