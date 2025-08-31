//go:build amd64 || arm64

package main

import (
	"net/url"

	"github.com/containers/podman/v5/internal/localapi"
)

func getMachineConn(connectionURI string, parsedConnection *url.URL) (string, error) {
	mc, machineProvider, err := localapi.FindMachineByPort(connectionURI, parsedConnection)
	if err != nil {
		return "", err
	}

	podmanSocket, podmanPipe, err := mc.ConnectionInfo(machineProvider.VMType())
	if err != nil {
		return "", err
	}
	return extractConnectionString(podmanSocket, podmanPipe)
}
