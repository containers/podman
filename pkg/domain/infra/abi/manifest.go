// +build ABISupport

package abi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	buildahUtil "github.com/containers/buildah/util"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports/alltransports"
	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"

	"github.com/pkg/errors"
)

// ManifestCreate implements logic for creating manifest lists via ImageEngine
func (ir *ImageEngine) ManifestCreate(ctx context.Context, names, images []string, opts entities.ManifestCreateOptions) (string, error) {
	fullNames, err := buildahUtil.ExpandNames(names, "", ir.Libpod.SystemContext(), ir.Libpod.GetStore())
	if err != nil {
		return "", errors.Wrapf(err, "error encountered while expanding image name %q", names)
	}
	imageID, err := libpodImage.CreateManifestList(ir.Libpod.ImageRuntime(), *ir.Libpod.SystemContext(), fullNames, images, opts.All)
	if err != nil {
		return imageID, err
	}
	return imageID, err
}

// ManifestInspect returns the content of a manifest list or image
func (ir *ImageEngine) ManifestInspect(ctx context.Context, name string) ([]byte, error) {
	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	_, err := alltransports.ParseImageName(name)
	if err != nil {
		_, err = alltransports.ParseImageName(dockerPrefix + name)
		if err != nil {
			return nil, errors.Errorf("invalid image reference %q", name)
		}
	}
	image, err := ir.Libpod.ImageRuntime().New(ctx, name, "", "", nil, nil, libpodImage.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		return nil, errors.Wrapf(err, "reading image %q", name)
	}

	list, err := image.InspectManifest()
	if err != nil {
		return nil, errors.Wrapf(err, "loading manifest %q", name)
	}
	buf, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		return buf, errors.Wrapf(err, "error rendering manifest for display")
	}
	return buf, nil
}

// ManifestAdd adds images to the manifest list
func (ir *ImageEngine) ManifestAdd(ctx context.Context, opts entities.ManifestAddOptions) (string, error) {
	imageSpec := opts.Images[0]
	listImageSpec := opts.Images[1]
	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	_, err := alltransports.ParseImageName(imageSpec)
	if err != nil {
		_, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", dockerPrefix, imageSpec))
		if err != nil {
			return "", errors.Errorf("invalid image reference %q", imageSpec)
		}
	}
	listImage, err := ir.Libpod.ImageRuntime().NewFromLocal(listImageSpec)
	if err != nil {
		return "", errors.Wrapf(err, "error retriving local image from image name %s", listImageSpec)
	}

	manifestAddOpts := libpodImage.ManifestAddOpts{
		All:       opts.All,
		Arch:      opts.Arch,
		Features:  opts.Features,
		Images:    opts.Images,
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
	listID, err := listImage.AddManifest(*ir.Libpod.SystemContext(), manifestAddOpts)
	if err != nil {
		return listID, err
	}
	return listID, nil
}
