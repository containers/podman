//go:build (amd64 && !remote) || (arm64 && !remote)
// +build amd64,!remote arm64,!remote

package system

import (
	cmdMach "github.com/containers/podman/v4/cmd/podman/machine"
)

func resetMachine() error {
	provider, err := cmdMach.GetSystemProvider()
	if err != nil {
		return err
	}
	return provider.RemoveAndCleanMachines()
}
