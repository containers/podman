package abi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/containers/common/libimage"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ManifestCreate implements logic for creating manifest lists via ImageEngine
func (ir *ImageEngine) ManifestCreate(ctx context.Context, names []string, images []string, opts entities.ManifestCreateOptions) (string, error) {
	// FIXME: change the interface of manifest create `names []string` ->
	// `name string`.
	if len(names) == 0 {
		return "", errors.New("no name specified for creating a manifest list")
	}
	name := names[0]

	manifestList, err := ir.Libpod.LibimageRuntime().CreateManifestList(name)
	if err != nil {
		return "", err
	}

	addOptions := &libimage.ManifestListAddOptions{All: opts.All}
	for _, image := range images {
		if _, err := manifestList.Add(ctx, image, addOptions); err != nil {
			return "", err
		}
	}

	return manifestList.ID(), nil
}

// ManifestExists checks if a manifest list with the given name exists in local storage
func (ir *ImageEngine) ManifestExists(ctx context.Context, name string) (*entities.BoolReport, error) {
	image, _, err := ir.Libpod.LibimageRuntime().LookupImage(name, &libimage.LookupImageOptions{IgnorePlatform: true})
	if err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			return &entities.BoolReport{Value: false}, nil
		}
		return nil, err
	}

	isManifestList, err := image.IsManifestList(ctx)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: isManifestList}, nil
}

// ManifestInspect returns the content of a manifest list or image
func (ir *ImageEngine) ManifestInspect(ctx context.Context, name string) ([]byte, error) {
	// NOTE: we have to do a bit of a limbo here as `podman manifest
	// inspect foo` wants to do a remote-inspect of foo iff "foo" in the
	// containers storage is an ordinary image but not a manifest list.

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		switch errors.Cause(err) {
		// Do a remote inspect if there's no local image or if the
		// local image is not a manifest list.
		case storage.ErrImageUnknown, libimage.ErrNotAManifestList:
			return ir.remoteManifestInspect(ctx, name)

		default:
			return nil, err
		}
	}

	schema2List, err := manifestList.Inspect()
	if err != nil {
		return nil, err
	}

	rawSchema2List, err := json.Marshal(schema2List)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if err := json.Indent(&b, rawSchema2List, "", "    "); err != nil {
		return nil, errors.Wrapf(err, "error rendering manifest %s for display", name)
	}
	return b.Bytes(), nil
}

// inspect a remote manifest list.
func (ir *ImageEngine) remoteManifestInspect(ctx context.Context, name string) ([]byte, error) {
	sys := ir.Libpod.SystemContext()

	resolved, err := shortnames.Resolve(sys, name)
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

	for _, candidate := range resolved.PullCandidates {
		ref, err := alltransports.ParseImageName("docker://" + candidate.Value.String())
		if err != nil {
			return nil, err
		}
		src, err := ref.NewImageSource(ctx, sys)
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
	// FIXME: the name options below are *mandatory* arguments and should
	// be reflected as such in the signature.

	if len(opts.Images) < 2 {
		return "", errors.New("manifest add requires two images")
	}

	imageName := opts.Images[0]
	listName := opts.Images[1]

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(listName)
	if err != nil {
		return "", err
	}

	addOptions := &libimage.ManifestListAddOptions{
		All:                   opts.All,
		AuthFilePath:          opts.Authfile,
		CertDirPath:           opts.CertDir,
		InsecureSkipTLSVerify: opts.SkipTLSVerify,
		Username:              opts.Username,
		Password:              opts.Password,
	}

	instanceDigest, err := manifestList.Add(ctx, imageName, addOptions)
	if err != nil {
		return "", err
	}

	annotateOptions := &libimage.ManifestListAnnotateOptions{
		Architecture: opts.Arch,
		Features:     opts.Features,
		OS:           opts.OS,
		OSVersion:    opts.OSVersion,
		Variant:      opts.Variant,
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
		annotateOptions.Annotations = annotations
	}

	if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, names []string, opts entities.ManifestAnnotateOptions) (string, error) {
	// FIXME: the `names` are *mandatory* arguments and should be
	// reflected as such in the signature.

	if len(names) < 2 {
		return "", errors.New("manifest annotate requires two names")
	}

	listName := names[0]
	instanceDigest, err := digest.Parse(names[1])
	if err != nil {
		return "", errors.Errorf(`invalid image digest "%s": %v`, names[1], err)
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(listName)
	if err != nil {
		return "", err
	}

	annotateOptions := &libimage.ManifestListAnnotateOptions{
		Architecture: opts.Arch,
		Features:     opts.Features,
		OS:           opts.OS,
		OSVersion:    opts.OSVersion,
		Variant:      opts.Variant,
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
		annotateOptions.Annotations = annotations
	}

	if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestRemove removes specified digest from the specified manifest list
func (ir *ImageEngine) ManifestRemove(ctx context.Context, names []string) (string, error) {
	// FIXME: the `names` are *mandatory* arguments and should be
	// reflected as such in the signature.

	if len(names) < 2 {
		return "", errors.New("manifest remove requires two names")
	}

	listName := names[0]
	instanceDigest, err := digest.Parse(names[1])
	if err != nil {
		return "", errors.Errorf(`invalid image digest "%s": %v`, names[1], err)
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(listName)
	if err != nil {
		return "", err
	}

	if err := manifestList.RemoveInstance(instanceDigest); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestRm removes the specified manifest list from storage
func (ir *ImageEngine) ManifestRm(ctx context.Context, names []string) (report *entities.ImageRemoveReport, rmErrors []error) {
	return ir.Remove(ctx, names, entities.ImageRemoveOptions{})
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, name, destination string, opts entities.ImagePushOptions) (string, error) {
	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", errors.Wrapf(err, "error retrieving local image from image name %s", name)
	}

	var manifestType string
	if opts.Format != "" {
		switch opts.Format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return "", errors.Errorf("unknown format %q. Choose one of the supported formats: 'oci' or 'v2s2'", opts.Format)
		}
	}

	pushOptions := &libimage.ManifestListPushOptions{}
	pushOptions.AuthFilePath = opts.Authfile
	pushOptions.CertDirPath = opts.CertDir
	pushOptions.Username = opts.Username
	pushOptions.Password = opts.Password
	pushOptions.ImageListSelection = cp.CopySpecificImages
	pushOptions.ManifestMIMEType = manifestType
	pushOptions.RemoveSignatures = opts.RemoveSignatures
	pushOptions.SignBy = opts.SignBy

	if opts.All {
		pushOptions.ImageListSelection = cp.CopyAllImages
	}
	if !opts.Quiet {
		pushOptions.Writer = os.Stderr
	}

	manDigest, err := manifestList.Push(ctx, destination, pushOptions)
	if err != nil {
		return "", err
	}

	if opts.Rm {
		if _, err := ir.Libpod.GetStore().DeleteImage(manifestList.ID(), true); err != nil {
			return "", errors.Wrap(err, "error removing manifest after push")
		}
	}

	return manDigest.String(), err
}
