//go:build !remote

package libimage

import (
	"context"
	"time"

	"github.com/containers/storage"
	"github.com/sirupsen/logrus"
)

// ImageDiskUsage reports the total size of an image.  That is the size
type ImageDiskUsage struct {
	// Number of containers using the image.
	Containers int
	// ID of the image.
	ID string
	// Repository of the image.
	Repository string
	// Tag of the image.
	Tag string
	// Created time stamp.
	Created time.Time
	// The amount of space that an image shares with another one (i.e. their common data).
	SharedSize int64
	// The the amount of space that is only used by a given image.
	UniqueSize int64
	// Sum of shared an unique size.
	Size int64
}

// DiskUsage calculates the disk usage for each image in the local containers
// storage.  Note that a single image may yield multiple usage reports, one for
// each repository tag.
func (r *Runtime) DiskUsage(ctx context.Context) ([]ImageDiskUsage, int64, error) {
	images, layers, err := r.getImagesAndLayers()
	if err != nil {
		return nil, -1, err
	}

	var totalSize int64
	layerMap := make(map[string]*storage.Layer)
	for _, layer := range layers {
		layerMap[layer.ID] = &layer
		if layer.UncompressedSize == -1 {
			// size is unknown, we must manually diff the layer size which
			// can be quite slow as it might have to walk all files
			size, err := r.store.DiffSize("", layer.ID)
			if err != nil {
				return nil, -1, err
			}
			// cache the size now
			layer.UncompressedSize = size
		}
		// count the total layer size here so we know we only count each layer once
		totalSize += layer.UncompressedSize
	}

	// First walk all images to count how often each layer is used.
	// This is done so we know if the size for an image is shared between
	// images that use the same layer or unique.
	layerCount := make(map[string]int)
	for _, image := range images {
		walkImageLayers(image, layerMap, func(layer *storage.Layer) {
			// Increment the count for each layer visit
			layerCount[layer.ID] += 1
		})
	}

	// Now that we actually have all the info walk again to add the sizes.
	var allUsages []ImageDiskUsage
	for _, image := range images {
		usages, err := diskUsageForImage(ctx, image, layerMap, layerCount, &totalSize)
		if err != nil {
			return nil, -1, err
		}
		allUsages = append(allUsages, usages...)
	}
	return allUsages, totalSize, err
}

// diskUsageForImage returns the disk-usage baseistics for the specified image.
func diskUsageForImage(ctx context.Context, image *Image, layerMap map[string]*storage.Layer, layerCount map[string]int, totalSize *int64) ([]ImageDiskUsage, error) {
	if err := image.isCorrupted(ctx, ""); err != nil {
		return nil, err
	}

	base := ImageDiskUsage{
		ID:         image.ID(),
		Created:    image.Created(),
		Repository: "<none>",
		Tag:        "<none>",
	}

	walkImageLayers(image, layerMap, func(layer *storage.Layer) {
		// If the layer used by more than one image it shares its size
		if layerCount[layer.ID] > 1 {
			base.SharedSize += layer.UncompressedSize
		} else {
			base.UniqueSize += layer.UncompressedSize
		}
	})

	// FIXME: The ImageSize() function calls BigDataSize() instead which
	// does try to lookup this map as well but also falls back to reading
	// the full big data files into memory for no apparent reason.
	// Can we trust that the map always has the data we care about?
	// Is there ever a case where this matters if the map does not have all the data?
	for _, size := range image.storageImage.BigDataSizes {
		base.UniqueSize += size
		*totalSize += size
	}

	base.Size = base.SharedSize + base.UniqueSize

	// Number of containers using the image.
	containers, err := image.Containers()
	if err != nil {
		return nil, err
	}
	base.Containers = len(containers)

	repoTags, err := image.NamedRepoTags()
	if err != nil {
		return nil, err
	}

	if len(repoTags) == 0 {
		return []ImageDiskUsage{base}, nil
	}

	pairs, err := ToNameTagPairs(repoTags)
	if err != nil {
		return nil, err
	}

	results := make([]ImageDiskUsage, len(pairs))
	for i, pair := range pairs {
		res := base
		res.Repository = pair.Name
		res.Tag = pair.Tag
		results[i] = res
	}

	return results, nil
}

// walkImageLayers walks all layers in an image and calls the given function for each layer.
func walkImageLayers(image *Image, layerMap map[string]*storage.Layer, f func(layer *storage.Layer)) {
	// Layers are walked recursively until it has no parent which means we reached the end.
	// We must account for the fact that an image might have several top layers when id mappings are used.
	layers := append([]string{image.storageImage.TopLayer}, image.storageImage.MappedTopLayers...)
	for _, layerID := range layers {
		for layerID != "" {
			layer := layerMap[layerID]
			if layer == nil {
				logrus.Errorf("Local Storage is corrupt, layer %q missing from the storage", layerID)
				break
			}
			f(layer)
			// Set the layer for the next iteration, parent is empty if we reach the end.
			// FIXME: This assumes there are no loops, is that a guarantee c/storage gives us?
			layerID = layer.Parent
		}
	}
}
