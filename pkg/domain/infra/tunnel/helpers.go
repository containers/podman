package tunnel

import (
	"context"
	"strings"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

func getContainersByContext(contextWithConnection context.Context, all bool, namesOrIds []string) ([]libpod.ListContainer, error) {
	var (
		cons []libpod.ListContainer
	)
	if all && len(namesOrIds) > 0 {
		return nil, errors.New("cannot lookup containers and all")
	}
	c, err := containers.List(contextWithConnection, nil, &bindings.PTrue, nil, nil, nil, &bindings.PTrue)
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
