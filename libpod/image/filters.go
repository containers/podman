package image

import (
	"context"
	"strings"
	"time"

	"github.com/containers/libpod/pkg/inspect"
)

// ResultFilter is a mock function for image filtering
type ResultFilter func(*Image) bool

// Filter is a function to determine whether an image is included in
// command output. Images to be outputted are tested using the function. A true
// return will include the image, a false return will exclude it.
type Filter func(*Image, *inspect.ImageData) bool

// CreatedBeforeFilter allows you to filter on images created before
// the given time.Time
func CreatedBeforeFilter(createTime time.Time) ResultFilter {
	return func(i *Image) bool {
		return i.Created().Before(createTime)
	}
}

// CreatedAfterFilter allows you to filter on images created after
// the given time.Time
func CreatedAfterFilter(createTime time.Time) ResultFilter {
	return func(i *Image) bool {
		return i.Created().After(createTime)
	}
}

// DanglingFilter allows you to filter images for dangling images
func DanglingFilter() ResultFilter {
	return func(i *Image) bool {
		return i.Dangling()
	}
}

// LabelFilter allows you to filter by images labels key and/or value
func LabelFilter(ctx context.Context, labelfilter string) ResultFilter {
	// We need to handle both label=key and label=key=value
	return func(i *Image) bool {
		var value string
		splitFilter := strings.Split(labelfilter, "=")
		key := splitFilter[0]
		if len(splitFilter) > 1 {
			value = splitFilter[1]
		}
		labels, err := i.Labels(ctx)
		if err != nil {
			return false
		}
		if len(strings.TrimSpace(labels[key])) > 0 && len(strings.TrimSpace(value)) == 0 {
			return true
		}
		return labels[key] == value
	}
}

// OutputImageFilter allows you to filter by an a specific image name
func OutputImageFilter(userImage *Image) ResultFilter {
	return func(i *Image) bool {
		return userImage.ID() == i.ID()
	}
}

// FilterImages filters images using a set of predefined filter funcs
func FilterImages(images []*Image, filters []ResultFilter) []*Image {
	var filteredImages []*Image
	for _, image := range images {
		include := true
		for _, filter := range filters {
			include = include && filter(image)
		}
		if include {
			filteredImages = append(filteredImages, image)
		}
	}
	return filteredImages
}
