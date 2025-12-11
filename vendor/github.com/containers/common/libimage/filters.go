//go:build !remote
// +build !remote

package libimage

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	filtersPkg "github.com/containers/common/pkg/filters"
	"github.com/containers/common/pkg/timetype"
	"github.com/containers/image/v5/docker/reference"
	"github.com/sirupsen/logrus"
)

// filterFunc is a prototype for a positive image filter.  Returning `true`
// indicates that the image matches the criteria.
type filterFunc func(*Image) (bool, error)

// Apply the specified filters.  At least one filter of each key must apply.
func (i *Image) applyFilters(filters map[string][]filterFunc) (bool, error) {
	matches := false
	for key := range filters { // and
		matches = false
		for _, filter := range filters[key] { // or
			var err error
			matches, err = filter(i)
			if err != nil {
				// Some images may have been corrupted in the
				// meantime, so do an extra check and make the
				// error non-fatal (see containers/podman/issues/12582).
				if errCorrupted := i.isCorrupted(""); errCorrupted != nil {
					logrus.Errorf(errCorrupted.Error())
					return false, nil
				}
				return false, err
			}
			if matches {
				break
			}
		}
		if !matches {
			return false, nil
		}
	}
	return matches, nil
}

// filterImages returns a slice of images which are passing all specified
// filters.
func (r *Runtime) filterImages(ctx context.Context, images []*Image, options *ListImagesOptions) ([]*Image, error) {
	if len(options.Filters) == 0 || len(images) == 0 {
		return images, nil
	}

	filters, err := r.compileImageFilters(ctx, options)
	if err != nil {
		return nil, err
	}
	result := []*Image{}
	for i := range images {
		match, err := images[i].applyFilters(filters)
		if err != nil {
			return nil, err
		}
		if match {
			result = append(result, images[i])
		}
	}
	return result, nil
}

// compileImageFilters creates `filterFunc`s for the specified filters.  The
// required format is `key=value` with the following supported keys:
//
//	after, since, before, containers, dangling, id, label, readonly, reference, intermediate
func (r *Runtime) compileImageFilters(ctx context.Context, options *ListImagesOptions) (map[string][]filterFunc, error) {
	logrus.Tracef("Parsing image filters %s", options.Filters)

	var tree *layerTree
	getTree := func() (*layerTree, error) {
		if tree == nil {
			t, err := r.layerTree(nil)
			if err != nil {
				return nil, err
			}
			tree = t
		}
		return tree, nil
	}

	filters := map[string][]filterFunc{}
	duplicate := map[string]string{}
	for _, f := range options.Filters {
		var key, value string
		var filter filterFunc
		negate := false
		split := strings.SplitN(f, "!=", 2)
		if len(split) == 2 {
			negate = true
		} else {
			split = strings.SplitN(f, "=", 2)
			if len(split) != 2 {
				return nil, fmt.Errorf("invalid image filter %q: must be in the format %q", f, "filter=value or filter!=value")
			}
		}

		key = split[0]
		value = split[1]
		switch key {
		case "after", "since":
			img, err := r.time(key, value)
			if err != nil {
				return nil, err
			}
			key = "since"
			filter = filterAfter(img.Created())

		case "before":
			img, err := r.time(key, value)
			if err != nil {
				return nil, err
			}
			filter = filterBefore(img.Created())

		case "containers":
			if err := r.containers(duplicate, key, value, options.IsExternalContainerFunc); err != nil {
				return nil, err
			}
			filter = filterContainers(value, options.IsExternalContainerFunc)

		case "dangling":
			dangling, err := r.bool(duplicate, key, value)
			if err != nil {
				return nil, err
			}
			t, err := getTree()
			if err != nil {
				return nil, err
			}

			filter = filterDangling(ctx, dangling, t)

		case "id":
			filter = filterID(value)

		case "digest":
			f, err := filterDigest(value)
			if err != nil {
				return nil, err
			}
			filter = f

		case "intermediate":
			intermediate, err := r.bool(duplicate, key, value)
			if err != nil {
				return nil, err
			}
			t, err := getTree()
			if err != nil {
				return nil, err
			}

			filter = filterIntermediate(ctx, intermediate, t)

		case "label":
			filter = filterLabel(ctx, value)
		case "readonly":
			readOnly, err := r.bool(duplicate, key, value)
			if err != nil {
				return nil, err
			}
			filter = filterReadOnly(readOnly)

		case "manifest":
			manifest, err := r.bool(duplicate, key, value)
			if err != nil {
				return nil, err
			}
			filter = filterManifest(ctx, manifest)

		case "reference":
			filter = filterReferences(r, value)

		case "until":
			until, err := r.until(value)
			if err != nil {
				return nil, err
			}
			filter = filterBefore(until)

		default:
			return nil, fmt.Errorf("unsupported image filter %q", key)
		}
		if negate {
			filter = negateFilter(filter)
		}
		filters[key] = append(filters[key], filter)
	}

	return filters, nil
}

func negateFilter(f filterFunc) filterFunc {
	return func(img *Image) (bool, error) {
		b, err := f(img)
		return !b, err
	}
}

func (r *Runtime) containers(duplicate map[string]string, key, value string, externalFunc IsExternalContainerFunc) error {
	if exists, ok := duplicate[key]; ok && exists != value {
		return fmt.Errorf("specifying %q filter more than once with different values is not supported", key)
	}
	duplicate[key] = value
	switch value {
	case "false", "true":
	case "external":
		if externalFunc == nil {
			return fmt.Errorf("libimage error: external containers filter without callback")
		}
	default:
		return fmt.Errorf("unsupported value %q for containers filter", value)
	}
	return nil
}

func (r *Runtime) until(value string) (time.Time, error) {
	var until time.Time
	ts, err := timetype.GetTimestamp(value, time.Now())
	if err != nil {
		return until, err
	}
	seconds, nanoseconds, err := timetype.ParseTimestamps(ts, 0)
	if err != nil {
		return until, err
	}
	return time.Unix(seconds, nanoseconds), nil
}

func (r *Runtime) time(key, value string) (*Image, error) {
	img, _, err := r.LookupImage(value, nil)
	if err != nil {
		return nil, fmt.Errorf("could not find local image for filter %q=%q: %w", key, value, err)
	}
	return img, nil
}

func (r *Runtime) bool(duplicate map[string]string, key, value string) (bool, error) {
	if exists, ok := duplicate[key]; ok && exists != value {
		return false, fmt.Errorf("specifying %q filter more than once with different values is not supported", key)
	}
	duplicate[key] = value
	set, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("non-boolean value %q for %s filter: %w", key, value, err)
	}
	return set, nil
}

// filterManifest filters whether or not the image is a manifest list
func filterManifest(ctx context.Context, value bool) filterFunc {
	return func(img *Image) (bool, error) {
		isManifestList, err := img.IsManifestList(ctx)
		if err != nil {
			return false, err
		}
		return isManifestList == value, nil
	}
}

// filterReferences creates a reference filter for matching the specified value.
func filterReferences(r *Runtime, value string) filterFunc {
	lookedUp, _, _ := r.LookupImage(value, nil)
	return func(img *Image) (bool, error) {
		if lookedUp != nil {
			if lookedUp.ID() == img.ID() {
				return true, nil
			}
		}

		refs, err := img.NamesReferences()
		if err != nil {
			return false, err
		}

		for _, ref := range refs {
			refString := ref.String() // FQN with tag/digest
			candidates := []string{refString}

			// Split the reference into 3 components (twice if digested/tagged):
			// 1) Fully-qualified reference
			// 2) Without domain
			// 3) Without domain and path
			if named, isNamed := ref.(reference.Named); isNamed {
				candidates = append(candidates,
					reference.Path(named),                           // path/name without tag/digest (Path() removes it)
					refString[strings.LastIndex(refString, "/")+1:]) // name with tag/digest

				trimmedString := reference.TrimNamed(named).String()
				if refString != trimmedString {
					tagOrDigest := refString[len(trimmedString):]
					candidates = append(candidates,
						trimmedString,                     // FQN without tag/digest
						reference.Path(named)+tagOrDigest, // path/name with tag/digest
						trimmedString[strings.LastIndex(trimmedString, "/")+1:]) // name without tag/digest
				}
			}

			for _, candidate := range candidates {
				// path.Match() is also used by Docker's reference.FamiliarMatch().
				matched, _ := path.Match(value, candidate)
				if matched {
					return true, nil
				}
			}
		}

		return false, nil
	}
}

// filterLabel creates a label for matching the specified value.
func filterLabel(ctx context.Context, value string) filterFunc {
	return func(img *Image) (bool, error) {
		labels, err := img.Labels(ctx)
		if err != nil {
			return false, err
		}
		return filtersPkg.MatchLabelFilters([]string{value}, labels), nil
	}
}

// filterAfter creates an after filter for matching the specified value.
func filterAfter(value time.Time) filterFunc {
	return func(img *Image) (bool, error) {
		return img.Created().After(value), nil
	}
}

// filterBefore creates a before filter for matching the specified value.
func filterBefore(value time.Time) filterFunc {
	return func(img *Image) (bool, error) {
		return img.Created().Before(value), nil
	}
}

// filterReadOnly creates a readonly filter for matching the specified value.
func filterReadOnly(value bool) filterFunc {
	return func(img *Image) (bool, error) {
		return img.IsReadOnly() == value, nil
	}
}

// filterContainers creates a container filter for matching the specified value.
func filterContainers(value string, fn IsExternalContainerFunc) filterFunc {
	return func(img *Image) (bool, error) {
		ctrs, err := img.Containers()
		if err != nil {
			return false, err
		}
		if value != "external" {
			boolValue := value == "true"
			return (len(ctrs) > 0) == boolValue, nil
		}

		// Check whether all associated containers are external ones.
		for _, c := range ctrs {
			isExternal, err := fn(c)
			if err != nil {
				return false, fmt.Errorf("checking if %s is an external container in filter: %w", c, err)
			}
			if !isExternal {
				return isExternal, nil
			}
		}
		return true, nil
	}
}

// filterDangling creates a dangling filter for matching the specified value.
func filterDangling(ctx context.Context, value bool, tree *layerTree) filterFunc {
	return func(img *Image) (bool, error) {
		isDangling, err := img.isDangling(ctx, tree)
		if err != nil {
			return false, err
		}
		return isDangling == value, nil
	}
}

// filterID creates an image-ID filter for matching the specified value.
func filterID(value string) filterFunc {
	return func(img *Image) (bool, error) {
		return strings.HasPrefix(img.ID(), value), nil
	}
}

// filterDigest creates a digest filter for matching the specified value.
func filterDigest(value string) (filterFunc, error) {
	if !strings.HasPrefix(value, "sha256:") {
		return nil, fmt.Errorf("invalid value %q for digest filter", value)
	}
	return func(img *Image) (bool, error) {
		return img.containsDigestPrefix(value), nil
	}, nil
}

// filterIntermediate creates an intermediate filter for images.  An image is
// considered to be an intermediate image if it is dangling (i.e., no tags) and
// has no children (i.e., no other image depends on it).
func filterIntermediate(ctx context.Context, value bool, tree *layerTree) filterFunc {
	return func(img *Image) (bool, error) {
		isIntermediate, err := img.isIntermediate(ctx, tree)
		if err != nil {
			return false, err
		}
		return isIntermediate == value, nil
	}
}
