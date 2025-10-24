//go:build !remote

package tunnel

import "github.com/containers/podman/v6/internal/domain/entities"

var _ entities.TestingEngine = &TestingEngine{}
