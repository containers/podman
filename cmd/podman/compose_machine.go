//go:build amd64 || arm64

package main

import (
	"net/url"

	"github.com/containers/podman/v5/internal/local_utils"
)

func getMachineConn(connectionURI string, parsedConnection *url.URL) (string, error) {
	mc, machineProvider, err := local_utils.FindMachineByPort(connectionURI, parsedConnection)
	if err != nil {
		return "", err
	}

	podmanSocket, podmanPipe, err := mc.ConnectionInfo(machineProvider.VMType())
	if err != nil {
		return "", err
	}
	return extractConnectionString(podmanSocket, podmanPipe)
}
