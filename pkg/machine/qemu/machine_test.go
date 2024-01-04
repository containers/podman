//go:build (amd64 && !windows) || (arm64 && !windows)

package qemu

import (
	"testing"

	"github.com/containers/podman/v4/pkg/machine/qemu/command"
	"github.com/stretchr/testify/require"
)

func TestEditCmd(t *testing.T) {
	vm := new(MachineVM)
	vm.CmdLine = command.QemuCmd{"command", "-flag", "value"}

	vm.editCmdLine("-flag", "newvalue")
	vm.editCmdLine("-anotherflag", "anothervalue")

	require.Equal(t, vm.CmdLine.Build(), []string{"command", "-flag", "newvalue", "-anotherflag", "anothervalue"})
}
