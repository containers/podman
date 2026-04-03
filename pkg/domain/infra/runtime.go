package infra

import (
	"context"

	"github.com/containers/podman/v6/pkg/bindings"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"go.podman.io/image/v5/pkg/cli/basetls/tlsdetails"
)

// For the meaning of "WithoutLock", compare runtime_tunnel.go:newConnection()
func newConnectionWithoutLock(ctx context.Context, facts *entities.PodmanConfig) (context.Context, error) {
	// Doing this here means we typically parse the file twice, once when called by NewContainerEngine
	// and once when called by NewImageEngine.
	// Alternatively, we could have the top-level caller parse the file, and pass us a baseTLSConfig.
	baseTLSConfig, err := tlsdetails.BaseTLSFromOptionalFile(facts.TLSDetailsFile)
	if err != nil {
		return nil, err
	}
	return bindings.NewConnectionWithOptions(ctx, bindings.Options{
		URI:           facts.URI,
		Identity:      facts.Identity,
		TLSCertFile:   facts.TLSCertFile,
		TLSKeyFile:    facts.TLSKeyFile,
		TLSCAFile:     facts.TLSCAFile,
		BaseTLSConfig: baseTLSConfig.TLSConfig(),
		Machine:       facts.MachineMode,
	})
}
