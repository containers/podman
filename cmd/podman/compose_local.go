//go:build !remote

// Compose requires running against a socket.  For native Linux clients (!remote),
// we can talk directly to the local Podman socket.  For remote clients, we need
// to run against podman-machine which is only available on amd64 and arm64.

package main

import "github.com/containers/podman/v5/cmd/podman/registry"

// composeDockerHost returns the value to be set in the DOCKER_HOST environment
// variable.
func composeDockerHost() (string, error) {
	return registry.DefaultAPIAddress(), nil
}
