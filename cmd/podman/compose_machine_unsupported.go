//go:build !(amd64 || arm64)

package main

import (
	"errors"
	"net/url"

	"github.com/containers/common/pkg/config"
)

func getMachineConn(connection *config.Connection, parsedConnection *url.URL) (string, error) {
	return "", errors.New("podman machine not supported on this architecture")
}
