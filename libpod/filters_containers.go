package libpod

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/libpod/pkg/util"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// type (
// 	// Filterable defines items to be filtered for CLI and Varlink
// 	FilterableContainer interface {
// 		Ancestors() []string
// 		CreatedTime() time.Time
// 		ExitCode() (int32, bool, error)
// 		ID() string
// 		Labels() map[string]string
// 		Mounts() []specs.Mount
// 		Name() string
// 		State() (libpod.ContainerStatus, error)
// 		Dangling() bool
// 	}
// )

// AncestorFilter used to filter on target ancestor
func AncestorFilter(target string) (Filter, error) {
	return func(f Filterable) bool {
		m, ok := f.(interface{ Ancestors() []string })
		if ok {
			for _, ancestor := range m.Ancestors() {
				if strings.Contains(ancestor, target) {
					return true
				}
			}
		}
		return false
	}, nil
}

// ExitedFilter used to filterable containers on target exit code
func ExitedFilter(target int32) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ ExitCode() (int32, bool, error) }); ok {
			code, exited, err := m.ExitCode()
			return err == nil && exited == true && code == target
		}
		return false
	}, nil
}

// StatusFilter used to filterable containers on target status
func StatusFilter(target string) (Filter, error) {
	if !util.StringInSlice(target, []string{"created", "restarting", "running", "paused", "exited", "unknown"}) {
		return nil, errors.Errorf("%s is not a valid status", target)
	}
	state := ""
	return func(f Filterable) bool {
		if m, ok := f.(interface{ State() (ContainerStatus, error) }); ok {
			status, _ := m.State()
			if status == ContainerStateConfigured {
				state = "created"
			} else {
				state = status.String()
			}
			return state == target
		}
		return false
	}, nil
}

// MountFilter used to filterable containers on used mounts
func MountFilter(target string) (Filter, error) {
	var targets = strings.Split(target, ":")
	source := targets[0]

	dest := ""
	if len(targets) == 2 {
		dest = targets[1]
	}

	return func(f Filterable) bool {
		if m, ok := f.(interface{ Mounts() []specs.Mount }); ok {
			for _, mount := range m.Mounts() {
				if dest != "" && (mount.Source == source && mount.Destination == dest) {
					return true
				}
				if dest == "" && mount.Source == source {
					return true
				}
			}
		}
		return false
	}, nil
}

// LatestOrAllContainers tries to return the correct list of containers
// depending if --all, --latest or <container-id> is used.
// It requires the Context (c) and the Runtime (runtime). As different
// commands are using different container state for the --all option
// the desired state has to be specified in filterState. If no filterable
// is desired a -1 can be used to get all containers. For a better
// error message, if the filterable fails, a corresponding verb can be
// specified which will then appear in the error message.
func LatestOrAllContainers(c *cliconfig.PodmanCommand, runtime *adapter.LocalRuntime, target ContainerStatus, verb string) ([]*adapter.Container, error) {
	var containers []*adapter.Container
	var lastError error
	var err error

	if c.Bool("all") {
		if target != -1 {
			var filters []Filter
			statusFilter, e := StatusFilter(target.String())
			if e != nil {
				return nil, errors.Wrapf(e, "unable to get filter (%s) containers", target)
			}
			filters = append(filters, statusFilter)
			containers, err = runtime.GetContainers(filters...)
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
		for _, id := range c.InputArgs {
			ctr, err := runtime.LookupContainer(id)
			if err != nil {
				if lastError != nil {
					fmt.Fprintln(os.Stderr, lastError)
				}
				lastError = errors.Wrapf(err, "unable to find container %s", ctr)
			}
			if ctr != nil {
				// This is here to make sure this does not return [<nil>] but only nil
				containers = append(containers, ctr)
			}
		}
	}

	return containers, lastError
}
