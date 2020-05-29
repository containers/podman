// +build ABISupport

package abi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/buildah/manifests"
	buildahUtil "github.com/containers/buildah/util"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"

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
		return "", errors.Wrapf(err, "error retrieving local image from image name %s", listImageSpec)
	}

	manifestAddOpts := libpodImage.ManifestAddOpts{
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
	listID, err := listImage.AddManifest(*ir.Libpod.SystemContext(), manifestAddOpts)
	if err != nil {
		return listID, err
	}
	return listID, nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, names []string, opts entities.ManifestAnnotateOptions) (string, error) {
	listImage, err := ir.Libpod.ImageRuntime().NewFromLocal(names[0])
	if err != nil {
		return "", errors.Wrapf(err, "error retreiving local image from image name %s", names[0])
	}
	digest, err := digest.Parse(names[1])
	if err != nil {
		return "", errors.Errorf(`invalid image digest "%s": %v`, names[1], err)
	}
	manifestAnnotateOpts := libpodImage.ManifestAnnotateOpts{
		Arch:       opts.Arch,
		Features:   opts.Features,
		OS:         opts.OS,
		OSFeatures: opts.OSFeatures,
		OSVersion:  opts.OSVersion,
		Variant:    opts.Variant,
	}
	if len(opts.Annotation) > 0 {
		annotations := make(map[string]string)
		for _, annotationSpec := range opts.Annotation {
			spec := strings.SplitN(annotationSpec, "=", 2)
			if len(spec) != 2 {
				return "", errors.Errorf("no value given for annotation %q", spec[0])
			}
			annotations[spec[0]] = spec[1]
		}
		manifestAnnotateOpts.Annotation = annotations
	}
	updatedListID, err := listImage.AnnotateManifest(*ir.Libpod.SystemContext(), digest, manifestAnnotateOpts)
	if err == nil {
		return fmt.Sprintf("%s: %s", updatedListID, digest.String()), nil
	}
	return "", err
}

// ManifestRemove removes specified digest from the specified manifest list
func (ir *ImageEngine) ManifestRemove(ctx context.Context, names []string) (string, error) {
	instanceDigest, err := digest.Parse(names[1])
	if err != nil {
		return "", errors.Errorf(`invalid image digest "%s": %v`, names[1], err)
	}
	listImage, err := ir.Libpod.ImageRuntime().NewFromLocal(names[0])
	if err != nil {
		return "", errors.Wrapf(err, "error retriving local image from image name %s", names[0])
	}
	updatedListID, err := listImage.RemoveManifest(instanceDigest)
	if err == nil {
		return fmt.Sprintf("%s :%s\n", updatedListID, instanceDigest.String()), nil
	}
	return "", err
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, names []string, opts entities.ManifestPushOptions) error {
	listImage, err := ir.Libpod.ImageRuntime().NewFromLocal(names[0])
	if err != nil {
		return errors.Wrapf(err, "error retriving local image from image name %s", names[0])
	}
	dest, err := alltransports.ParseImageName(names[1])
	if err != nil {
		return err
	}
	var manifestType string
	if opts.Format != "" {
		switch opts.Format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return errors.Errorf("unknown format %q. Choose one of the supported formats: 'oci' or 'v2s2'", opts.Format)
		}
	}

	// Set the system context.
	sys := ir.Libpod.SystemContext()
	if sys != nil {
		sys = &types.SystemContext{}
	}
	sys.AuthFilePath = opts.Authfile
	sys.DockerInsecureSkipTLSVerify = opts.SkipTLSVerify

	if opts.Username != "" && opts.Password != "" {
		sys.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.Username,
			Password: opts.Password,
		}
	}

	options := manifests.PushOptions{
		Store:              ir.Libpod.GetStore(),
		SystemContext:      sys,
		ImageListSelection: cp.CopySpecificImages,
		Instances:          nil,
		RemoveSignatures:   opts.RemoveSignatures,
		SignBy:             opts.SignBy,
		ManifestType:       manifestType,
	}
	if opts.All {
		options.ImageListSelection = cp.CopyAllImages
	}
	if !opts.Quiet {
		options.ReportWriter = os.Stderr
	}
	digest, err := listImage.PushManifest(dest, options)
	if err == nil && opts.Purge {
		_, err = ir.Libpod.GetStore().DeleteImage(listImage.ID(), true)
	}
	if opts.DigestFile != "" {
		if err = ioutil.WriteFile(opts.DigestFile, []byte(digest.String()), 0644); err != nil {
			return buildahUtil.GetFailureCause(err, errors.Wrapf(err, "failed to write digest to file %q", opts.DigestFile))
		}
	}
	return err
}
