//go:build !remote

package abi

import "github.com/containers/podman/v5/internal/domain/entities"

var _ entities.TestingEngine = &TestingEngine{}
