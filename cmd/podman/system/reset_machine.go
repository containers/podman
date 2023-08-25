//go:build (amd64 && !remote) || (arm64 && !remote)
// +build amd64,!remote arm64,!remote

package system

import (
	p "github.com/containers/podman/v4/pkg/machine/provider"
)

func resetMachine() error {
	provider, err := p.Get()
	if err != nil {
		return err
	}
	return provider.RemoveAndCleanMachines()
}
