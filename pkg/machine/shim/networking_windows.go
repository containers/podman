package shim

import (
	"fmt"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

func setupMachineSockets(mc *vmconfigs.MachineConfig, dirs *define.MachineDirs) ([]string, string, machine.APIForwardingState, error) {
	machinePipe := env.WithPodmanPrefix(mc.Name)
	if !machine.PipeNameAvailable(machinePipe, machine.MachineNameWait) {
		return nil, "", 0, fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}
	sockets := []string{machine.NamedPipePrefix + machinePipe}
	state := machine.MachineLocal

	if machine.PipeNameAvailable(machine.GlobalNamedPipe, machine.GlobalNameWait) {
		sockets = append(sockets, machine.NamedPipePrefix+machine.GlobalNamedPipe)
		state = machine.DockerGlobal
	}

	return sockets, sockets[len(sockets)-1], state, nil
}
