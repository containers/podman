package tarball

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/types"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// ConfigUpdater is an interface that ImageReferences for "tarball" images also
// implement.  It can be used to set values for a configuration, and to set
// image annotations which will be present in the images returned by the
// reference's NewImage() or NewImageSource() methods.
type ConfigUpdater interface {
	ConfigUpdate(config imgspecv1.Image, annotations map[string]string) error
}

type tarballReference struct {
	transport   types.ImageTransport
	config      imgspecv1.Image
	annotations map[string]string
	filenames   []string
	stdin       []byte
}

// ConfigUpdate updates the image's default configuration and adds annotations
// which will be visible in source images created using this reference.
func (r *tarballReference) ConfigUpdate(config imgspecv1.Image, annotations map[string]string) error {
	r.config = config
	if r.annotations == nil {
		r.annotations = make(map[string]string)
	}
	for k, v := range annotations {
		r.annotations[k] = v
	}
	return nil
}

func (r *tarballReference) Transport() types.ImageTransport {
	return r.transport
}

func (r *tarballReference) StringWithinTransport() string {
	return strings.Join(r.filenames, ":")
}

func (r *tarballReference) DockerReference() reference.Named {
	return nil
}

func (r *tarballReference) PolicyConfigurationIdentity() string {
	return ""
}

func (r *tarballReference) PolicyConfigurationNamespaces() []string {
	return nil
}

func (r *tarballReference) NewImage(ctx *types.SystemContext) (types.Image, error) {
	src, err := r.NewImageSource(ctx)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(src)
	if err != nil {
		src.Close()
		return nil, err
	}
	return img, nil
}

func (r *tarballReference) DeleteImage(ctx *types.SystemContext) error {
	for _, filename := range r.filenames {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing %q: %v", filename, err)
		}
	}
	return nil
}

func (r *tarballReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return nil, fmt.Errorf("destination not implemented yet")
}
