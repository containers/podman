//go:build !remote
// +build !remote

package infra

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/domain/infra/tunnel"
)

// NewContainerEngine factory provides a libpod runtime for container-related operations
func NewContainerEngine(facts *entities.PodmanConfig) (entities.ContainerEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnectionWithIdentity(context.Background(), facts.URI, facts.Identity, facts.MachineMode)
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
		ctx, err := bindings.NewConnectionWithIdentity(context.Background(), facts.URI, facts.Identity, facts.MachineMode)
		return &tunnel.ImageEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}

// NewSystemEngine factory provides a libpod runtime for specialized system operations
func NewSystemEngine(setup entities.EngineSetup, facts *entities.PodmanConfig) (entities.SystemEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		var r *libpod.Runtime
		var err error
		switch setup {
		case entities.NormalMode:
			r, err = GetRuntime(context.Background(), facts.FlagSet, facts)
		case entities.RenumberMode:
			r, err = GetRuntimeRenumber(context.Background(), facts.FlagSet, facts)
		case entities.ResetMode:
			r, err = GetRuntimeReset(context.Background(), facts.FlagSet, facts)
		case entities.MigrateMode:
			name, flagErr := facts.FlagSet.GetString("new-runtime")
			if flagErr != nil {
				return nil, flagErr
			}
			r, err = GetRuntimeMigrate(context.Background(), facts.FlagSet, facts, name)
		case entities.NoFDsMode:
			r, err = GetRuntimeDisableFDs(context.Background(), facts.FlagSet, facts)
		}
		return &abi.SystemEngine{Libpod: r}, err
	case entities.TunnelMode:
		return nil, fmt.Errorf("tunnel system runtime not supported")
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
