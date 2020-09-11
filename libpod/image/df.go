package image

import (
	"context"
	"time"

	"github.com/containers/image/v5/docker/reference"
)

// DiskUsageStat gives disk-usage statistics for a specific image.
type DiskUsageStat struct {
	// ID of the image.
	ID string
	// Repository of the first recorded name of the image.
	Repository string
	// Tag of the first recorded name of the image.
	Tag string
	// Created is the creation time of the image.
	Created time.Time
	// SharedSize is the amount of space shared with another image.
	SharedSize uint64
	// UniqueSize is the amount of space used only by this image.
	UniqueSize uint64
	// Size is the total size of the image (i.e., the sum of the shared and
	// unique size).
	Size uint64
	// Number of containers using the image.
	Containers int
}

// DiskUsage returns disk-usage statistics for the specified slice of images.
func (ir *Runtime) DiskUsage(ctx context.Context, images []*Image) ([]DiskUsageStat, error) {
	stats := make([]DiskUsageStat, len(images))

	// Build a layerTree to quickly compute (and cache!) parent/child
	// relations.
	tree, err := ir.layerTree()
	if err != nil {
		return nil, err
	}

	// Calculate the stats for each image.
	for i, img := range images {
		stat, err := diskUsageForImage(ctx, img, tree)
		if err != nil {
			return nil, err
		}
		stats[i] = *stat
	}

	return stats, nil
}

// diskUsageForImage returns the disk-usage statistics for the spcified image.
func diskUsageForImage(ctx context.Context, image *Image, tree *layerTree) (*DiskUsageStat, error) {
	stat := DiskUsageStat{
		ID:      image.ID(),
		Created: image.Created(),
	}

	// Repository and tag.
	var name, repository, tag string
	for _, n := range image.Names() {
		if len(n) > 0 {
			name = n
			break
		}
	}
	if len(name) > 0 {
		named, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, err
		}
		repository = named.Name()
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			tag = tagged.Tag()
		}
	} else {
		repository = "<none>"
		tag = "<none>"
	}
	stat.Repository = repository
	stat.Tag = tag

	// Shared, unique and total size.
	parent, err := tree.parent(ctx, image)
	if err != nil {
		return nil, err
	}
	childIDs, err := tree.children(ctx, image, false)
	if err != nil {
		return nil, err
	}
	// Optimistically set unique size to the full size of the image.
	size, err := image.Size(ctx)
	if err != nil {
		return nil, err
	}
	stat.UniqueSize = *size

	if len(childIDs) > 0 {
		// If we have children, we share everything.
		stat.SharedSize = stat.UniqueSize
		stat.UniqueSize = 0
	} else if parent != nil {
		// If we have no children but a parent, remove the parent
		// (shared) size from the unique one.
		size, err := parent.Size(ctx)
		if err != nil {
			return nil, err
		}
		stat.UniqueSize -= *size
		stat.SharedSize = *size
	}

	stat.Size = stat.SharedSize + stat.UniqueSize

	// Number of containers using the image.
	containers, err := image.Containers()
	if err != nil {
		return nil, err
	}
	stat.Containers = len(containers)

	return &stat, nil
}
