//build !remoteclient

package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
)

// getAllOrLatestContainers tries to return the correct list of containers
// depending if --all, --latest or <container-id> is used.
// It requires the Context (c) and the Runtime (runtime). As different
// commands are using different container state for the --all option
// the desired state has to be specified in filterState. If no filter
// is desired a -1 can be used to get all containers. For a better
// error message, if the filter fails, a corresponding verb can be
// specified which will then appear in the error message.
func getAllOrLatestContainers(c *cliconfig.PodmanCommand, runtime *libpod.Runtime, filterState define.ContainerStatus, verb string) ([]*libpod.Container, error) {
	var containers []*libpod.Container
	var lastError error
	var err error
	if c.Bool("all") {
		if filterState != -1 {
			var filterFuncs []libpod.ContainerFilter
			filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
				state, _ := c.State()
				return state == filterState
			})
			containers, err = runtime.GetContainers(filterFuncs...)
		} else {
			containers, err = runtime.GetContainers()
		}
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get %s containers", verb)
		}
	} else if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get latest container")
		}
		containers = append(containers, lastCtr)
	} else {
		args := c.InputArgs
		for _, i := range args {
			container, err := runtime.LookupContainer(i)
			if err != nil {
				if lastError != nil {
					fmt.Fprintln(os.Stderr, lastError)
				}
				lastError = errors.Wrapf(err, "unable to find container %s", i)
			}
			if container != nil {
				// This is here to make sure this does not return [<nil>] but only nil
				containers = append(containers, container)
			}
		}
	}

	return containers, lastError
}
