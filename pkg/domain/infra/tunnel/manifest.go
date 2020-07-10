package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/bindings/manifests"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

// ManifestCreate implements manifest create via ImageEngine
func (ir *ImageEngine) ManifestCreate(ctx context.Context, names, images []string, opts entities.ManifestCreateOptions) (string, error) {
	imageID, err := manifests.Create(ir.ClientCxt, names, images, &opts.All)
	if err != nil {
		return imageID, errors.Wrapf(err, "error creating manifest")
	}
	return imageID, err
}

// ManifestInspect returns contents of manifest list with given name
func (ir *ImageEngine) ManifestInspect(ctx context.Context, name string) ([]byte, error) {
	list, err := manifests.Inspect(ir.ClientCxt, name)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting content of manifest list or image %s", name)
	}

	buf, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		return buf, errors.Wrapf(err, "error rendering manifest for display")
	}
	return buf, err
}

// ManifestAdd adds images to the manifest list
func (ir *ImageEngine) ManifestAdd(ctx context.Context, opts entities.ManifestAddOptions) (string, error) {
	manifestAddOpts := image.ManifestAddOpts{
		All:       opts.All,
		Arch:      opts.Arch,
		Features:  opts.Features,
		Images:    opts.Images,
		OS:        opts.OS,
		OSVersion: opts.OSVersion,
		Variant:   opts.Variant,
	}
	if len(opts.Annotation) != 0 {
		annotations := make(map[string]string)
		for _, annotationSpec := range opts.Annotation {
			spec := strings.SplitN(annotationSpec, "=", 2)
			if len(spec) != 2 {
				return "", errors.Errorf("no value given for annotation %q", spec[0])
			}
			annotations[spec[0]] = spec[1]
		}
		manifestAddOpts.Annotation = annotations
	}
	listID, err := manifests.Add(ir.ClientCxt, opts.Images[1], manifestAddOpts)
	if err != nil {
		return listID, errors.Wrapf(err, "error adding to manifest list %s", opts.Images[1])
	}
	return listID, nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, names []string, opts entities.ManifestAnnotateOptions) (string, error) {
	return "", errors.New("not implemented")
}

// ManifestRemove removes the digest from manifest list
func (ir *ImageEngine) ManifestRemove(ctx context.Context, names []string) (string, error) {
	updatedListID, err := manifests.Remove(ir.ClientCxt, names[0], names[1])
	if err != nil {
		return updatedListID, errors.Wrapf(err, "error removing from manifest %s", names[0])
	}
	return fmt.Sprintf("%s :%s\n", updatedListID, names[1]), nil
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, names []string, opts entities.ManifestPushOptions) error {
	_, err := manifests.Push(ir.ClientCxt, names[0], &names[1], &opts.All)
	return err
}
