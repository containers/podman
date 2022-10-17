package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ListenUnix follows stdlib net.Listen() API, providing a unix listener for given path
//
//	ListenUnix will delete and create files/directories as needed
func ListenUnix(network string, path string) (net.Listener, error) {
	// set up custom listener for API server
	err := os.MkdirAll(filepath.Dir(path), 0770)
	if err != nil {
		return nil, fmt.Errorf("api.ListenUnix() failed to create %s: %w", filepath.Dir(path), err)
	}
	os.Remove(path)

	listener, err := net.Listen(network, path)
	if err != nil {
		return nil, fmt.Errorf("api.ListenUnix() failed to create net.Listen(%s, %s): %w", network, path, err)
	}

	_, err = os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("net.Listen(%s, %s) failed to report the failure to create socket: %w", network, path, err)
	}

	return listener, nil
}
