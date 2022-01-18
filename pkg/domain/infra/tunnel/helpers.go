package tunnel

import (
	"context"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/pods"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/pkg/errors"
)

// FIXME: the `ignore` parameter is very likely wrong here as it should rather
//        be used on *errors* from operations such as remove.
func getContainersByContext(contextWithConnection context.Context, all, ignore bool, namesOrIDs []string) ([]entities.ListContainer, error) {
	ctrs, _, err := getContainersAndInputByContext(contextWithConnection, all, ignore, namesOrIDs)
	return ctrs, err
}

func getContainersAndInputByContext(contextWithConnection context.Context, all, ignore bool, namesOrIDs []string) ([]entities.ListContainer, []string, error) {
	if all && len(namesOrIDs) > 0 {
		return nil, nil, errors.New("cannot lookup containers and all")
	}
	options := new(containers.ListOptions).WithAll(true).WithSync(true)
	allContainers, err := containers.List(contextWithConnection, options)
	if err != nil {
		return nil, nil, err
	}
	rawInputs := []string{}
	if all {
		for i := range allContainers {
			rawInputs = append(rawInputs, allContainers[i].ID)
		}

		return allContainers, rawInputs, err
	}

	// Note: it would be nicer if the lists endpoint would support that as
	// we could use the libpod backend for looking up containers rather
	// than risking diverging the local and remote lookups.
	//
	// A `--filter nameOrId=abc` that can be specified multiple times would
	// be awesome to have.
	filtered := []entities.ListContainer{}
	for _, nameOrID := range namesOrIDs {
		// First determine if the container exists by doing an inspect.
		// Inspect takes supports names and IDs and let's us determine
		// a containers full ID.
		inspectData, err := containers.Inspect(contextWithConnection, nameOrID, new(containers.InspectOptions).WithSize(false))
		if err != nil {
			if ignore && errorhandling.Contains(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, nil, err
		}

		// Now we can do a full match of the ID to find the right
		// container. Note that we *really* need a full ID match to
		// prevent any ambiguities between IDs and names (see #7837).
		found := false
		for _, ctr := range allContainers {
			if ctr.ID == inspectData.ID {
				filtered = append(filtered, ctr)
				rawInputs = append(rawInputs, nameOrID)
				found = true
				break
			}
		}

		if !found && !ignore {
			return nil, nil, errors.Wrapf(define.ErrNoSuchCtr, "unable to find container %q", nameOrID)
		}
	}
	return filtered, rawInputs, nil
}

func getPodsByContext(contextWithConnection context.Context, all bool, namesOrIDs []string) ([]*entities.ListPodsReport, error) {
	if all && len(namesOrIDs) > 0 {
		return nil, errors.New("cannot lookup specific pods and all")
	}

	allPods, err := pods.List(contextWithConnection, nil)
	if err != nil {
		return nil, err
	}
	if all {
		return allPods, nil
	}

	filtered := []*entities.ListPodsReport{}
	// Note: it would be nicer if the lists endpoint would support that as
	// we could use the libpod backend for looking up pods rather than
	// risking diverging the local and remote lookups.
	//
	// A `--filter nameOrId=abc` that can be specified multiple times would
	// be awesome to have.
	for _, nameOrID := range namesOrIDs {
		// First determine if the pod exists by doing an inspect.
		// Inspect takes supports names and IDs and let's us determine
		// a containers full ID.
		inspectData, err := pods.Inspect(contextWithConnection, nameOrID, nil)
		if err != nil {
			if errorhandling.Contains(err, define.ErrNoSuchPod) {
				return nil, errors.Wrapf(define.ErrNoSuchPod, "unable to find pod %q", nameOrID)
			}
			return nil, err
		}

		// Now we can do a full match of the ID to find the right pod.
		// Note that we *really* need a full ID match to prevent any
		// ambiguities between IDs and names (see #7837).
		found := false
		for _, pod := range allPods {
			if pod.Id == inspectData.ID {
				filtered = append(filtered, pod)
				found = true
				break
			}
		}

		if !found {
			return nil, errors.Wrapf(define.ErrNoSuchPod, "unable to find pod %q", nameOrID)
		}
	}
	return filtered, nil
}
