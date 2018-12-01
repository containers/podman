package shared

import (
	"fmt"
	"github.com/pkg/errors"

	"github.com/containers/libpod/libpod/image"
)

// Prune removes all unnamed and unused images from the local store
func Prune(ir *image.Runtime) error {
	pruneImages, err := ir.GetPruneImages()
	if err != nil {
		return err
	}

	for _, i := range pruneImages {
		if err := i.Remove(true); err != nil {
			return errors.Wrapf(err, "failed to remove %s", i.ID())
		}
		fmt.Println(i.ID())
	}
	return nil
}
