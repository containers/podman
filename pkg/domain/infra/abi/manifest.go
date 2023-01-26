package abi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"errors"

	"github.com/containers/common/libimage"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	envLib "github.com/containers/podman/v4/pkg/env"
	"github.com/containers/storage"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// ManifestCreate implements logic for creating manifest lists via ImageEngine
func (ir *ImageEngine) ManifestCreate(ctx context.Context, name string, images []string, opts entities.ManifestCreateOptions) (string, error) {
	if len(name) == 0 {
		return "", errors.New("no name specified for creating a manifest list")
	}

	manifestList, err := ir.Libpod.LibimageRuntime().CreateManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateName) && opts.Amend {
			amendList, amendErr := ir.Libpod.LibimageRuntime().LookupManifestList(name)
			if amendErr != nil {
				return "", err
			}
			manifestList = amendList
		} else {
			return "", err
		}
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
	_, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			return &entities.BoolReport{Value: false}, nil
		}
		return nil, err
	}

	return &entities.BoolReport{Value: true}, nil
}

// ManifestInspect returns the content of a manifest list or image
func (ir *ImageEngine) ManifestInspect(ctx context.Context, name string, opts entities.ManifestInspectOptions) ([]byte, error) {
	// NOTE: we have to do a bit of a limbo here as `podman manifest
	// inspect foo` wants to do a remote-inspect of foo iff "foo" in the
	// containers storage is an ordinary image but not a manifest list.

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) || errors.Is(err, libimage.ErrNotAManifestList) {
			// Do a remote inspect if there's no local image or if the
			// local image is not a manifest list.
			return ir.remoteManifestInspect(ctx, name, opts)
		}

		return nil, err
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
		return nil, fmt.Errorf("rendering manifest %s for display: %w", name, err)
	}
	return b.Bytes(), nil
}

// inspect a remote manifest list.
func (ir *ImageEngine) remoteManifestInspect(ctx context.Context, name string, opts entities.ManifestInspectOptions) ([]byte, error) {
	sys := ir.Libpod.SystemContext()

	sys.DockerInsecureSkipTLSVerify = opts.SkipTLSVerify
	if opts.SkipTLSVerify == types.OptionalBoolTrue {
		sys.OCIInsecureSkipTLSVerify = true
	}

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
			// FIXME should we use multierror package instead?

			// we want the new line here so ignore the linter
			latestErr = fmt.Errorf("tried %v\n: %w", e, latestErr)
		}
	}

	for _, candidate := range resolved.PullCandidates {
		ref, err := alltransports.ParseImageName("docker://" + candidate.Value.String())
		if err != nil {
			return nil, err
		}
		src, err := ref.NewImageSource(ctx, sys)
		if err != nil {
			appendErr(fmt.Errorf("reading image %q: %w", transports.ImageName(ref), err))
			continue
		}
		defer src.Close()

		manifestBytes, manifestType, err := src.GetManifest(ctx, nil)
		if err != nil {
			appendErr(fmt.Errorf("loading manifest %q: %w", transports.ImageName(ref), err))
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
		logrus.Warnf("The manifest type %s is not a manifest list but a single image.", manType)
		schema2Manifest, err := manifest.Schema2FromManifest(result)
		if err != nil {
			return nil, fmt.Errorf("parsing manifest blob %q as a %q: %w", string(result), manType, err)
		}
		if result, err = schema2Manifest.Serialize(); err != nil {
			return nil, err
		}
	default:
		listBlob, err := manifest.ListFromBlob(result, manType)
		if err != nil {
			return nil, fmt.Errorf("parsing manifest blob %q as a %q: %w", string(result), manType, err)
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
		return nil, fmt.Errorf("rendering manifest %s for display: %w", name, err)
	}
	return b.Bytes(), nil
}

// ManifestAdd adds images to the manifest list
func (ir *ImageEngine) ManifestAdd(ctx context.Context, name string, images []string, opts entities.ManifestAddOptions) (string, error) {
	if len(images) < 1 {
		return "", errors.New("manifest add requires at least one image")
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
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

	for _, image := range images {
		instanceDigest, err := manifestList.Add(ctx, image, addOptions)
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
					return "", fmt.Errorf("no value given for annotation %q", spec[0])
				}
				annotations[spec[0]] = spec[1]
			}
			opts.Annotations = envLib.Join(opts.Annotations, annotations)
		}
		annotateOptions.Annotations = opts.Annotations

		if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
			return "", err
		}
	}
	return manifestList.ID(), nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, name, image string, opts entities.ManifestAnnotateOptions) (string, error) {
	instanceDigest, err := digest.Parse(image)
	if err != nil {
		return "", fmt.Errorf(`invalid image digest "%s": %v`, image, err)
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
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
				return "", fmt.Errorf("no value given for annotation %q", spec[0])
			}
			annotations[spec[0]] = spec[1]
		}
		opts.Annotations = envLib.Join(opts.Annotations, annotations)
	}
	annotateOptions.Annotations = opts.Annotations

	if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestRemoveDigest removes specified digest from the specified manifest list
func (ir *ImageEngine) ManifestRemoveDigest(ctx context.Context, name, image string) (string, error) {
	instanceDigest, err := digest.Parse(image)
	if err != nil {
		return "", fmt.Errorf(`invalid image digest "%s": %v`, image, err)
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
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
	return ir.Remove(ctx, names, entities.ImageRemoveOptions{LookupManifest: true})
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, name, destination string, opts entities.ImagePushOptions) (string, error) {
	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", fmt.Errorf("retrieving local image from image name %s: %w", name, err)
	}

	var manifestType string
	if opts.Format != "" {
		switch opts.Format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return "", fmt.Errorf("unknown format %q. Choose one of the supported formats: 'oci' or 'v2s2'", opts.Format)
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
	pushOptions.Signers = opts.Signers
	pushOptions.SignBy = opts.SignBy
	pushOptions.SignPassphrase = opts.SignPassphrase
	pushOptions.SignBySigstorePrivateKeyFile = opts.SignBySigstorePrivateKeyFile
	pushOptions.SignSigstorePrivateKeyPassphrase = opts.SignSigstorePrivateKeyPassphrase
	pushOptions.InsecureSkipTLSVerify = opts.SkipTLSVerify
	pushOptions.Writer = opts.Writer

	compressionFormat := opts.CompressionFormat
	if compressionFormat == "" {
		config, err := ir.Libpod.GetConfigNoCopy()
		if err != nil {
			return "", err
		}
		compressionFormat = config.Engine.CompressionFormat
	}
	if compressionFormat != "" {
		algo, err := compression.AlgorithmByName(compressionFormat)
		if err != nil {
			return "", err
		}
		pushOptions.CompressionFormat = &algo
	}

	if opts.All {
		pushOptions.ImageListSelection = cp.CopyAllImages
	}
	if !opts.Quiet && pushOptions.Writer == nil {
		pushOptions.Writer = os.Stderr
	}

	manDigest, err := manifestList.Push(ctx, destination, pushOptions)
	if err != nil {
		return "", err
	}

	if opts.Rm {
		rmOpts := &libimage.RemoveImagesOptions{LookupManifest: true}
		if _, rmErrors := ir.Libpod.LibimageRuntime().RemoveImages(ctx, []string{manifestList.ID()}, rmOpts); len(rmErrors) > 0 {
			return "", fmt.Errorf("removing manifest after push: %w", rmErrors[0])
		}
	}

	return manDigest.String(), err
}
