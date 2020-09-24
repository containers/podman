package image

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v2/pkg/inspect"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// IntermediateFilter returns filter for intermediate images (i.e., images
// with children and no tags).
func (ir *Runtime) IntermediateFilter(ctx context.Context, images []*Image) (ResultFilter, error) {
	tree, err := ir.layerTree()
	if err != nil {
		return nil, err
	}
	return func(i *Image) bool {
		if len(i.Names()) > 0 {
			return true
		}
		children, err := tree.children(ctx, i, false)
		if err != nil {
			logrus.Error(err.Error())
			return false
		}
		return len(children) == 0
	}, nil
}

// CreatedAfterFilter allows you to filter on images created after
// the given time.Time
func CreatedAfterFilter(createTime time.Time) ResultFilter {
	return func(i *Image) bool {
		return i.Created().After(createTime)
	}
}

// DanglingFilter allows you to filter images for dangling images
func DanglingFilter(danglingImages bool) ResultFilter {
	return func(i *Image) bool {
		if danglingImages {
			return i.Dangling()
		}
		return !i.Dangling()
	}
}

// ReadOnlyFilter allows you to filter images based on read/only and read/write
func ReadOnlyFilter(readOnly bool) ResultFilter {
	return func(i *Image) bool {
		if readOnly {
			return i.IsReadOnly()
		}
		return !i.IsReadOnly()
	}
}

// LabelFilter allows you to filter by images labels key and/or value
func LabelFilter(ctx context.Context, labelfilter string) ResultFilter {
	// We need to handle both label=key and label=key=value
	return func(i *Image) bool {
		var value string
		splitFilter := strings.SplitN(labelfilter, "=", 2)
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
	return func(i *Image) bool {
		if len(referenceFilter) < 1 {
			return true
		}
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

// IDFilter allows you to filter by image Id
func IDFilter(idFilter string) ResultFilter {
	return func(i *Image) bool {
		return i.ID() == idFilter
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

// createFilterFuncs returns an array of filter functions based on the user inputs
// and is later used to filter images for output
func (ir *Runtime) createFilterFuncs(filters []string, img *Image) ([]ResultFilter, error) {
	var filterFuncs []ResultFilter
	ctx := context.Background()
	for _, filter := range filters {
		splitFilter := strings.SplitN(filter, "=", 2)
		if len(splitFilter) < 2 {
			return nil, errors.Errorf("invalid filter syntax %s", filter)
		}
		switch splitFilter[0] {
		case "before":
			before, err := ir.NewFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image %s in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, CreatedBeforeFilter(before.Created()))
		case "since", "after":
			after, err := ir.NewFromLocal(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find image %s in local stores", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, CreatedAfterFilter(after.Created()))
		case "readonly":
			readonly, err := strconv.ParseBool(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "invalid filter readonly=%s", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, ReadOnlyFilter(readonly))
		case "dangling":
			danglingImages, err := strconv.ParseBool(splitFilter[1])
			if err != nil {
				return nil, errors.Wrapf(err, "invalid filter dangling=%s", splitFilter[1])
			}
			filterFuncs = append(filterFuncs, DanglingFilter(danglingImages))
		case "label":
			labelFilter := strings.Join(splitFilter[1:], "=")
			filterFuncs = append(filterFuncs, LabelFilter(ctx, labelFilter))
		case "reference":
			filterFuncs = append(filterFuncs, ReferenceFilter(ctx, splitFilter[1]))
		case "id":
			filterFuncs = append(filterFuncs, IDFilter(splitFilter[1]))
		default:
			return nil, errors.Errorf("invalid filter %s ", splitFilter[0])
		}
	}
	if img != nil {
		filterFuncs = append(filterFuncs, OutputImageFilter(img))
	}
	return filterFuncs, nil
}
