//go:build remote && (amd64 || arm64)

// Compose requires running against a socket.  For native Linux clients (!remote),
// we can talk directly to the local Podman socket.  For remote clients, we need
// to run against podman-machine which is only available on amd64 and arm64.

package main

import (
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

// composeDockerHost returns the value to be set in the DOCKER_HOST environment
// variable.
func composeDockerHost() (string, error) {
	// TODO need to add support for --connection and --url
	connection, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetConnection("", true)
	if err != nil {
		logrus.Info(err)
		switch runtime.GOOS {
		// If no default connection is set on Linux or FreeBSD,
		// we just use the local socket by default - just as
		// the remote client does.
		case "linux", "freebsd":
			return registry.DefaultAPIAddress(), nil
		// If there is no default connection on Windows or Mac
		// OS, we can safely assume that something went wrong.
		// A `podman machine init` will set the connection.
		default:
			return "", fmt.Errorf("cannot connect to a socket or via SSH: no default connection found: consider running `podman machine init`")
		}
	}

	parsedConnection, err := url.Parse(connection.URI)
	if err != nil {
		return "", fmt.Errorf("preparing connection to remote machine: %w", err)
	}

	// If the default connection does not point to a `podman
	// machine`, we cannot use a local path and need to use SSH.
	if !connection.IsMachine {
		// Compose doesn't like paths, so we optimistically
		// assume the presence of a Docker socket on the remote
		// machine which is the case for podman machines.
		return strings.TrimSuffix(connection.URI, parsedConnection.Path), nil
	}

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
