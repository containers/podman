package tunnel

import (
	"context"
	"strings"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/bindings/pods"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

func getContainersByContext(contextWithConnection context.Context, all bool, namesOrIds []string) ([]entities.ListContainer, error) {
	var (
		cons []entities.ListContainer
	)
	if all && len(namesOrIds) > 0 {
		return nil, errors.New("cannot lookup containers and all")
	}
	c, err := containers.List(contextWithConnection, nil, bindings.PTrue, nil, nil, nil, bindings.PTrue)
	if err != nil {
		return nil, err
	}
	if all {
		return c, err
	}
	for _, id := range namesOrIds {
		var found bool
		for _, con := range c {
			if id == con.ID || strings.HasPrefix(con.ID, id) || util.StringInSlice(id, con.Names) {
				cons = append(cons, con)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("unable to find container %q", id)
		}
	}
	return cons, nil
}

func getPodsByContext(contextWithConnection context.Context, all bool, namesOrIds []string) ([]*entities.ListPodsReport, error) {
	var (
		sPods []*entities.ListPodsReport
	)
	if all && len(namesOrIds) > 0 {
		return nil, errors.New("cannot lookup specific pods and all")
	}

	fPods, err := pods.List(contextWithConnection, nil)
	if err != nil {
		return nil, err
	}
	if all {
		return fPods, nil
	}
	for _, nameOrId := range namesOrIds {
		var found bool
		for _, f := range fPods {
			if f.Name == nameOrId || strings.HasPrefix(f.Id, nameOrId) {
				sPods = append(sPods, f)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Wrapf(define.ErrNoSuchPod, "unable to find pod %q", nameOrId)
		}
	}
	return sPods, nil
}
