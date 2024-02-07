//go:build remote

package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/tunnel"
)

var (
	connectionMutex = &sync.Mutex{}
	connection      *context.Context
)

func newConnection(uri string, identity, farmNodeName string, machine bool) (context.Context, error) {
	connectionMutex.Lock()
	defer connectionMutex.Unlock()

	// if farmNodeName given, then create a connection with the node so that we can send builds there
	if connection == nil || farmNodeName != "" {
		ctx, err := bindings.NewConnectionWithIdentity(context.Background(), uri, identity, machine)
		if err != nil {
			return ctx, err
		}
		connection = &ctx
	}
	return *connection, nil
}

func NewContainerEngine(facts *entities.PodmanConfig) (entities.ContainerEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		return nil, fmt.Errorf("direct runtime not supported")
	case entities.TunnelMode:
		ctx, err := newConnection(facts.URI, facts.Identity, "", facts.MachineMode)
		return &tunnel.ContainerEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}

// NewImageEngine factory provides a libpod runtime for image-related operations
func NewImageEngine(facts *entities.PodmanConfig) (entities.ImageEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		return nil, fmt.Errorf("direct image runtime not supported")
	case entities.TunnelMode:
		ctx, err := newConnection(facts.URI, facts.Identity, facts.FarmNodeName, facts.MachineMode)
		return &tunnel.ImageEngine{ClientCtx: ctx, FarmNode: tunnel.FarmNode{NodeName: facts.FarmNodeName}}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
