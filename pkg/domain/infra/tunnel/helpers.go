package tunnel

import (
	"context"
	"strings"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/bindings/containers"
	"github.com/containers/podman/v2/pkg/bindings/pods"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

func getContainersByContext(contextWithConnection context.Context, all, ignore bool, namesOrIDs []string) ([]entities.ListContainer, error) {
	var (
		cons []entities.ListContainer
	)
	if all && len(namesOrIDs) > 0 {
		return nil, errors.New("cannot lookup containers and all")
	}
	c, err := containers.List(contextWithConnection, nil, bindings.PTrue, nil, nil, bindings.PTrue)
	if err != nil {
		return nil, err
	}
	if all {
		return c, err
	}
	for _, id := range namesOrIDs {
		var found bool
		for _, con := range c {
			if id == con.ID || strings.HasPrefix(con.ID, id) || util.StringInSlice(id, con.Names) {
				cons = append(cons, con)
				found = true
				break
			}
		}
		if !found && !ignore {
			return nil, errors.Wrapf(define.ErrNoSuchCtr, "unable to find container %q", id)
		}
	}
	return cons, nil
}

func getPodsByContext(contextWithConnection context.Context, all bool, namesOrIDs []string) ([]*entities.ListPodsReport, error) {
	var (
		sPods []*entities.ListPodsReport
	)
	if all && len(namesOrIDs) > 0 {
		return nil, errors.New("cannot lookup specific pods and all")
	}

	fPods, err := pods.List(contextWithConnection, nil)
	if err != nil {
		return nil, err
	}
	if all {
		return fPods, nil
	}
	for _, nameOrID := range namesOrIDs {
		var found bool
		for _, f := range fPods {
			if f.Name == nameOrID || strings.HasPrefix(f.Id, nameOrID) {
				sPods = append(sPods, f)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Wrapf(define.ErrNoSuchPod, "unable to find pod %q", nameOrID)
		}
	}
	return sPods, nil
}
