package common

import (
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

func verifyExpose(expose []string) error {
	// add the expose ports from the user (--expose)
	// can be single or a range
	for _, expose := range expose {
		// support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		_, port := nat.SplitProtoPort(expose)
		// parse the start and end port and create a sequence of ports to expose
		// if expose a port, the start and end port are the same
		_, _, err := nat.ParsePortRange(port)
		if err != nil {
			return errors.Wrapf(err, "invalid range format for --expose: %s", expose)
		}
	}
	return nil
}
