//go:build arm64 && darwin

package applehv

import (
	"errors"
	"fmt"

	"github.com/containers/podman/v4/pkg/machine"
)

// VZMachineState is what the restful service in vfkit will return
type VZMachineState string

const (
	// Values that the machine can be in
	// "VirtualMachineStateStoppedVirtualMachineStateRunningVirtualMachineStatePausedVirtualMachineStateErrorVirtualMachineStateStartingVirtualMachineStatePausingVirtualMachineStateResumingVirtualMachineStateStopping"
	VZMachineStateStopped  VZMachineState = "VirtualMachineStateStopped"
	VZMachineStateRunning  VZMachineState = "VirtualMachineStateRunning"
	VZMachineStatePaused   VZMachineState = "VirtualMachineStatePaused"
	VZMachineStateError    VZMachineState = "VirtualMachineStateError"
	VZMachineStateStarting VZMachineState = "VirtualMachineStateStarting"
	VZMachineStatePausing  VZMachineState = "VirtualMachineStatePausing"
	VZMachineStateResuming VZMachineState = "VirtualMachineStateResuming"
	VZMachineStateStopping VZMachineState = "VirtualMachineStateStopping"
)

func ToMachineStatus(val string) (machine.Status, error) {
	switch val {
	case string(VZMachineStateRunning), string(VZMachineStatePausing), string(VZMachineStateResuming), string(VZMachineStateStopping), string(VZMachineStatePaused):
		return machine.Running, nil
	case string(VZMachineStateStopped):
		return machine.Stopped, nil
	case string(VZMachineStateStarting):
		return machine.Starting, nil
	case string(VZMachineStateError):
		return "", errors.New("machine is in error state")
	}
	return "", fmt.Errorf("unknown machine state: %s", val)
}
