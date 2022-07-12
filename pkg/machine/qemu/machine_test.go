//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditCmd(t *testing.T) {
	vm := new(MachineVM)
	vm.CmdLine = []string{"command", "-flag", "value"}

	vm.editCmdLine("-flag", "newvalue")
	vm.editCmdLine("-anotherflag", "anothervalue")

	require.Equal(t, vm.CmdLine, []string{"command", "-flag", "newvalue", "-anotherflag", "anothervalue"})
}
