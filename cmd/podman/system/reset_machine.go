//go:build (amd64 && !remote) || (arm64 && !remote)
// +build amd64,!remote arm64,!remote

package system

import (
	cmdMach "github.com/containers/podman/v4/cmd/podman/machine"
)

func resetMachine() error {
	provider := cmdMach.GetSystemDefaultProvider()
	return provider.RemoveAndCleanMachines()
}
