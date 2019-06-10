package imagefilters

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
)

// ResultFilter is a mock function for image filtering
type ResultFilter func(*adapter.ContainerImage) bool

// Filter is a function to determine whether an image is included in
// command output. Images to be outputted are tested using the function. A true
// return will include the image, a false return will exclude it.
type Filter func(*adapter.ContainerImage, *inspect.ImageData) bool

// CreatedBeforeFilter allows you to filter on images created before
// the given time.Time
func CreatedBeforeFilter(createTime time.Time) ResultFilter {
	return func(i *adapter.ContainerImage) bool {
		return i.Created().Before(createTime)
	}
}

// CreatedAfterFilter allows you to filter on images created after
// the given time.Time
func CreatedAfterFilter(createTime time.Time) ResultFilter {
	return func(i *adapter.ContainerImage) bool {
		return i.Created().After(createTime)
	}
}

// DanglingFilter allows you to filter images for dangling images
func DanglingFilter(danglingImages bool) ResultFilter {
	return func(i *adapter.ContainerImage) bool {
		if danglingImages {
			return i.Dangling()
		}
		return !i.Dangling()
	}
}

// LabelFilter allows you to filter by images labels key and/or value
func LabelFilter(ctx context.Context, labelfilter string) ResultFilter {
	// We need to handle both label=key and label=key=value
	return func(i *adapter.ContainerImage) bool {
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

// ReferenceFilter allows you to filter by image name
// Replacing all '/' with '|' so that filepath.Match() can work
// '|' character is not valid in image name, so this is safe
func ReferenceFilter(ctx context.Context, referenceFilter string) ResultFilter {
	filter := fmt.Sprintf("*%s*", referenceFilter)
	filter = strings.Replace(filter, "/", "|", -1)
	return func(i *adapter.ContainerImage) bool {
		for _, name := range i.Names() {
			newName := strings.Replace(name, "/", "|", -1)
			match, err := filepath.Match(filter, newName)
			if err != nil {
				logrus.Errorf("failed to match %s and %s, %q", name, referenceFilter, err)
			}
			if match {
				return true
			}
		}
		return false
	}
}

// OutputImageFilter allows you to filter by an a specific image name
func OutputImageFilter(userImage *adapter.ContainerImage) ResultFilter {
	return func(i *adapter.ContainerImage) bool {
		return userImage.ID() == i.ID()
	}
}

// FilterImages filters images using a set of predefined filter funcs
func FilterImages(images []*adapter.ContainerImage, filters []ResultFilter) []*adapter.ContainerImage {
	var filteredImages []*adapter.ContainerImage
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
