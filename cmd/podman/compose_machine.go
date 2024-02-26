//go:build amd64 || arm64

package main

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

func getMachineConn(connection *config.Connection, parsedConnection *url.URL) (string, error) {
	machineProvider, err := provider.Get()
	if err != nil {
		return "", fmt.Errorf("getting machine provider: %w", err)
	}
	dirs, err := machine.GetMachineDirs(machineProvider.VMType())
	if err != nil {
		return "", err
	}

	machineList, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		return "", fmt.Errorf("listing machines: %w", err)
	}

	// Now we know that the connection points to a machine and we
	// can find the machine by looking for the one with the
	// matching port.
	connectionPort, err := strconv.Atoi(parsedConnection.Port())
	if err != nil {
		return "", fmt.Errorf("parsing connection port: %w", err)
	}
	for _, item := range machineList {
		if connectionPort != item.SSH.Port {
			continue
		}

		state, err := machineProvider.State(item, false)
		if err != nil {
			return "", err
		}

		if state != define.Running {
			return "", fmt.Errorf("machine %s is not running but in state %s", item.Name, state)
		}

		// TODO This needs to be wired back in when all providers are complete
		// TODO Need someoone to plumb in the connection information below
		// if machineProvider.VMType() == define.WSLVirt || machineProvider.VMType() == define.HyperVVirt {
		// 	if info.ConnectionInfo.PodmanPipe == nil {
		// 		return "", errors.New("pipe of machine is not set")
		// 	}
		// 	return strings.Replace(info.ConnectionInfo.PodmanPipe.Path, `\\.\pipe\`, "npipe:////./pipe/", 1), nil
		// }
		// if info.ConnectionInfo.PodmanSocket == nil {
		// 	return "", errors.New("socket of machine is not set")
		// }
		// return "unix://" + info.ConnectionInfo.PodmanSocket.Path, nil
		return "", nil
	}

	return "", fmt.Errorf("could not find a matching machine for connection %q", connection.URI)
}
