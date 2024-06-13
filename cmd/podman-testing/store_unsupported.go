//go:build !linux || remote
// +build !linux remote

package main

import "github.com/containers/podman/v5/pkg/domain/entities"

const engineMode = entities.TunnelMode

func storeBefore() error {
	return nil
}

func storeAfter() error {
	return nil
}
