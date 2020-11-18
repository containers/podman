package server

import (
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// ListenUnix follows stdlib net.Listen() API, providing a unix listener for given path
//   ListenUnix will delete and create files/directories as needed
func ListenUnix(network string, path string) (net.Listener, error) {
	// setup custom listener for API server
	err := os.MkdirAll(filepath.Dir(path), 0770)
	if err != nil {
		return nil, errors.Wrapf(err, "api.ListenUnix() failed to create %s", filepath.Dir(path))
	}
	os.Remove(path)

	listener, err := net.Listen(network, path)
	if err != nil {
		return nil, errors.Wrapf(err, "api.ListenUnix() failed to create net.Listen(%s, %s)", network, path)
	}

	_, err = os.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "net.Listen(%s, %s) failed to report the failure to create socket", network, path)
	}

	return listener, nil
}
