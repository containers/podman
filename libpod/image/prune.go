package image

import (
	"context"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetPruneImages returns a slice of images that have no names/unused
func (ir *Runtime) GetPruneImages(all bool) ([]*Image, error) {
	var (
		pruneImages []*Image
	)
	allImages, err := ir.GetRWImages()
	if err != nil {
		return nil, err
	}
	for _, i := range allImages {
		if len(i.Names()) == 0 {
			pruneImages = append(pruneImages, i)
			continue
		}
		if all {
			containers, err := i.Containers()
			if err != nil {
				return nil, err
			}
			if len(containers) < 1 {
				pruneImages = append(pruneImages, i)
			}
		}
	}
	return pruneImages, nil
}

// PruneImages prunes dangling and optionally all unused images from the local
// image store
func (ir *Runtime) PruneImages(ctx context.Context, all bool) ([]string, error) {
	var prunedCids []string
	pruneImages, err := ir.GetPruneImages(all)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get images to prune")
	}
	for _, p := range pruneImages {
		if err := p.Remove(ctx, true); err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				logrus.Warnf("Failed to prune image %s as it is in use: %v", p.ID(), err)
				continue
			}
			return nil, errors.Wrap(err, "failed to prune image")
		}
		defer p.newImageEvent(events.Prune)
		prunedCids = append(prunedCids, p.ID())
	}
	return prunedCids, nil
}
