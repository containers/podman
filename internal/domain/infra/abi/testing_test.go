//go:build !remote && (linux || freebsd)

package abi

import "github.com/containers/podman/v6/internal/domain/entities"

var _ entities.TestingEngine = &TestingEngine{}
