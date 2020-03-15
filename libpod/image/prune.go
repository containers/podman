package image

import (
	"context"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/timetype"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func generatePruneFilterFuncs(filter, filterValue string) (ImageFilter, error) {
	switch filter {
	case "label":
		var filterArray = strings.SplitN(filterValue, "=", 2)
		var filterKey = filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(i *Image) bool {
			labels, err := i.Labels(context.Background())
			if err != nil {
				return false
			}
			for labelKey, labelValue := range labels {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil

	case "until":
		ts, err := timetype.GetTimestamp(filterValue, time.Now())
		if err != nil {
			return nil, err
		}
		seconds, nanoseconds, err := timetype.ParseTimestamps(ts, 0)
		if err != nil {
			return nil, err
		}
		until := time.Unix(seconds, nanoseconds)
		return func(i *Image) bool {
			if !until.IsZero() && i.Created().After((until)) {
				return true
			}
			return false
		}, nil

	}
	return nil, nil
}

// GetPruneImages returns a slice of images that have no names/unused
func (ir *Runtime) GetPruneImages(all bool, filterFuncs []ImageFilter) ([]*Image, error) {
	var (
		pruneImages []*Image
	)

	allImages, err := ir.GetRWImages()
	if err != nil {
		return nil, err
	}
	for _, i := range allImages {
		// filter the images based on this.
		for _, filterFunc := range filterFuncs {
			if !filterFunc(i) {
				continue
			}
		}

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
func (ir *Runtime) PruneImages(ctx context.Context, all bool, filter []string) ([]string, error) {
	var (
		prunedCids  []string
		filterFuncs []ImageFilter
	)
	for _, f := range filter {
		filterSplit := strings.SplitN(f, "=", 2)
		if len(filterSplit) < 2 {
			return nil, errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
		}

		generatedFunc, err := generatePruneFilterFuncs(filterSplit[0], filterSplit[1])
		if err != nil {
			return nil, errors.Wrapf(err, "invalid filter")
		}
		filterFuncs = append(filterFuncs, generatedFunc)
	}

	pruneImages, err := ir.GetPruneImages(all, filterFuncs)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get images to prune")
	}
	for _, p := range pruneImages {
		repotags, err := p.RepoTags()
		if err != nil {
			return nil, err
		}
		if err := p.Remove(ctx, true); err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				logrus.Warnf("Failed to prune image %s as it is in use: %v", p.ID(), err)
				continue
			}
			return nil, errors.Wrap(err, "failed to prune image")
		}
		defer p.newImageEvent(events.Prune)
		nameOrID := p.ID()
		if len(repotags) > 0 {
			nameOrID = repotags[0]
		}
		prunedCids = append(prunedCids, nameOrID)
	}
	return prunedCids, nil
}
