// +build remote

package infra

import (
	"errors"

	"github.com/containers/podman/v2/pkg/domain/entities"
)

// NewSystemEngine factory provides a libpod runtime for specialized system operations
func NewSystemEngine(setup entities.EngineSetup, facts *entities.PodmanConfig) (entities.SystemEngine, error) {
	return nil, errors.New("not implemented")
}
