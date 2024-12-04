//go:build amd64 || arm64

package main

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

func getMachineConn(connectionURI string, parsedConnection *url.URL) (string, error) {
	machineProvider, err := provider.Get()
	if err != nil {
		return "", fmt.Errorf("getting machine provider: %w", err)
	}
	dirs, err := env.GetMachineDirs(machineProvider.VMType())
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
	for _, mc := range machineList {
		if connectionPort != mc.SSH.Port {
			continue
		}

		state, err := machineProvider.State(mc, false)
		if err != nil {
			return "", err
		}

		if state != define.Running {
			return "", fmt.Errorf("machine %s is not running but in state %s", mc.Name, state)
		}

		podmanSocket, podmanPipe, err := mc.ConnectionInfo(machineProvider.VMType())
		if err != nil {
			return "", err
		}
		return extractConnectionString(podmanSocket, podmanPipe)
	}
	return "", fmt.Errorf("could not find a matching machine for connection %q", connectionURI)
}
