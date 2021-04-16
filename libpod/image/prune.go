package image

import (
	"context"
	"strconv"
	"strings"

	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func generatePruneFilterFuncs(filter, filterValue string) (ImageFilter, error) {
	switch filter {
	case "label":
		return func(i *Image) bool {
			labels, err := i.Labels(context.Background())
			if err != nil {
				return false
			}
			return util.MatchLabelFilters([]string{filterValue}, labels)
		}, nil

	case "until":
		until, err := util.ComputeUntilTimestamp([]string{filterValue})
		if err != nil {
			return nil, err
		}
		return func(i *Image) bool {
			if !until.IsZero() && i.Created().After((until)) {
				return true
			}
			return false
		}, nil
	case "dangling":
		danglingImages, err := strconv.ParseBool(filterValue)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid filter dangling=%s", filterValue)
		}
		return ImageFilter(DanglingFilter(danglingImages)), nil
	}
	return nil, nil
}

// GetPruneImages returns a slice of images that have no names/unused
func (ir *Runtime) GetPruneImages(ctx context.Context, all bool, filterFuncs []ImageFilter) ([]*Image, error) {
	var (
		pruneImages []*Image
	)

	allImages, err := ir.GetRWImages()
	if err != nil {
		return nil, err
	}

	tree, err := ir.layerTree()
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

		if all {
			containers, err := i.Containers()
			if err != nil {
				return nil, err
			}
			if len(containers) < 1 {
				pruneImages = append(pruneImages, i)
				continue
			}
		}

		// skip the cache (i.e., with parent) and intermediate (i.e.,
		// with children) images
		intermediate, err := tree.hasChildrenAndParent(ctx, i)
		if err != nil {
			return nil, err
		}
		if intermediate {
			continue
		}

		if i.Dangling() {
			pruneImages = append(pruneImages, i)
		}
	}
	return pruneImages, nil
}

// PruneImages prunes dangling and optionally all unused images from the local
// image store
func (ir *Runtime) PruneImages(ctx context.Context, all bool, filter []string) ([]*reports.PruneReport, error) {
	preports := make([]*reports.PruneReport, 0)
	filterFuncs := make([]ImageFilter, 0, len(filter))
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

	prev := 0
	for {
		toPrune, err := ir.GetPruneImages(ctx, all, filterFuncs)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get images to prune")
		}
		numImages := len(toPrune)
		if numImages == 0 || numImages == prev {
			// If there's nothing left to do, return.
			break
		}
		prev = numImages
		for _, img := range toPrune {
			repotags, err := img.RepoTags()
			if err != nil {
				return nil, err
			}
			nameOrID := img.ID()
			s, err := img.Size(ctx)
			imgSize := uint64(0)
			if err != nil {
				logrus.Warnf("Failed to collect image size for: %s, %s", nameOrID, err)
			} else {
				imgSize = *s
			}
			if err := img.Remove(ctx, false); err != nil {
				if errors.Cause(err) == storage.ErrImageUsedByContainer {
					logrus.Warnf("Failed to prune image %s as it is in use: %v.\nA container associated with containers/storage (e.g., Buildah, CRI-O, etc.) maybe associated with this image.\nUsing the rmi command with the --force option will remove the container and image, but may cause failures for other dependent systems.", img.ID(), err)
					continue
				}
				return nil, errors.Wrap(err, "failed to prune image")
			}
			defer img.newImageEvent(events.Prune)

			if len(repotags) > 0 {
				nameOrID = repotags[0]
			}

			preports = append(preports, &reports.PruneReport{
				Id:   nameOrID,
				Err:  nil,
				Size: uint64(imgSize),
			})
		}
	}
	return preports, nil
}
