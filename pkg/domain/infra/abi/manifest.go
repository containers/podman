// +build !remote

package abi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/buildah/manifests"
	buildahManifests "github.com/containers/buildah/pkg/manifests"
	"github.com/containers/buildah/util"
	buildahUtil "github.com/containers/buildah/util"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	libpodImage "github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

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
	if newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(name); err == nil {
		// return the manifest in local storage
		if list, err := newImage.InspectManifest(); err == nil {
			buf, err := json.MarshalIndent(list, "", "    ")
			if err != nil {
				return buf, errors.Wrapf(err, "error rendering manifest %s for display", name)
			}
			return buf, nil
			// no return if local image is not a list of images type
			// continue on getting valid manifest through remote serice
		} else if errors.Cause(err) != buildahManifests.ErrManifestTypeNotSupported {
			return nil, errors.Wrapf(err, "loading manifest %q", name)
		}
	}
	sc := ir.Libpod.SystemContext()
	refs, err := util.ResolveNameToReferences(ir.Libpod.GetStore(), sc, name)
	if err != nil {
		return nil, err
	}
	var (
		latestErr error
		result    []byte
		manType   string
		b         bytes.Buffer
	)
	appendErr := func(e error) {
		if latestErr == nil {
			latestErr = e
		} else {
			latestErr = errors.Wrapf(latestErr, "tried %v\n", e)
		}
	}
	for _, ref := range refs {
		src, err := ref.NewImageSource(ctx, sc)
		if err != nil {
			appendErr(errors.Wrapf(err, "reading image %q", transports.ImageName(ref)))
			continue
		}
		defer src.Close()

		manifestBytes, manifestType, err := src.GetManifest(ctx, nil)
		if err != nil {
			appendErr(errors.Wrapf(err, "loading manifest %q", transports.ImageName(ref)))
			continue
		}

		result = manifestBytes
		manType = manifestType
		break
	}
	if len(result) == 0 && latestErr != nil {
		return nil, latestErr
	}

	switch manType {
	case manifest.DockerV2Schema2MediaType:
		logrus.Warnf("Warning! The manifest type %s is not a manifest list but a single image.", manType)
		schema2Manifest, err := manifest.Schema2FromManifest(result)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing manifest blob %q as a %q", string(result), manType)
		}
		if result, err = schema2Manifest.Serialize(); err != nil {
			return nil, err
		}
	default:
		listBlob, err := manifest.ListFromBlob(result, manType)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing manifest blob %q as a %q", string(result), manType)
		}
		list, err := listBlob.ConvertToMIMEType(manifest.DockerV2ListMediaType)
		if err != nil {
			return nil, err
		}
		if result, err = list.Serialize(); err != nil {
			return nil, err
		}
	}

	if err = json.Indent(&b, result, "", "    "); err != nil {
		return nil, errors.Wrapf(err, "error rendering manifest %s for display", name)
	}
	return b.Bytes(), nil
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

	// Set the system context.
	sys := ir.Libpod.SystemContext()
	if sys != nil {
		sys = &types.SystemContext{}
	}
	sys.AuthFilePath = opts.Authfile
	sys.DockerInsecureSkipTLSVerify = opts.SkipTLSVerify
	sys.DockerCertPath = opts.CertDir

	if opts.Username != "" && opts.Password != "" {
		sys.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.Username,
			Password: opts.Password,
		}
	}

	listID, err := listImage.AddManifest(*sys, manifestAddOpts)
	if err != nil {
		return listID, err
	}
	return listID, nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, names []string, opts entities.ManifestAnnotateOptions) (string, error) {
	listImage, err := ir.Libpod.ImageRuntime().NewFromLocal(names[0])
	if err != nil {
		return "", errors.Wrapf(err, "error retrieving local image from image name %s", names[0])
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
		return "", errors.Wrapf(err, "error retrieving local image from image name %s", names[0])
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
		return errors.Wrapf(err, "error retrieving local image from image name %s", names[0])
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
	sys.DockerCertPath = opts.CertDir

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
