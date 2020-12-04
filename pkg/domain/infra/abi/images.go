package abi

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	libpodImage "github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	domainUtils "github.com/containers/podman/v2/pkg/domain/utils"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/trust"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SignatureStoreDir defines default directory to store signatures
const SignatureStoreDir = "/var/lib/containers/sigstore"

func (ir *ImageEngine) Exists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	_, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		if errors.Cause(err) == define.ErrMultipleImages {
			return &entities.BoolReport{Value: true}, nil
		} else {
			if errors.Cause(err) != define.ErrNoSuchImage {
				return nil, err
			}
		}
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) (*entities.ImagePruneReport, error) {
	results, err := ir.Libpod.ImageRuntime().PruneImages(ctx, opts.All, opts.Filter)
	if err != nil {
		return nil, err
	}

	report := entities.ImagePruneReport{
		Report: entities.Report{
			Id:  results,
			Err: nil,
		},
	}
	return &report, nil
}

func (ir *ImageEngine) History(ctx context.Context, nameOrID string, opts entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	image, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return nil, err
	}
	results, err := image.History(ctx)
	if err != nil {
		return nil, err
	}

	history := entities.ImageHistoryReport{
		Layers: make([]entities.ImageHistoryLayer, len(results)),
	}

	for i, layer := range results {
		history.Layers[i] = ToDomainHistoryLayer(layer)
	}
	return &history, nil
}

func (ir *ImageEngine) Mount(ctx context.Context, nameOrIDs []string, opts entities.ImageMountOptions) ([]*entities.ImageMountReport, error) {
	var (
		images []*image.Image
		err    error
	)
	if os.Geteuid() != 0 {
		if driver := ir.Libpod.StorageConfig().GraphDriverName; driver != "vfs" {
			// Do not allow to mount a graphdriver that is not vfs if we are creating the userns as part
			// of the mount command.
			return nil, errors.Errorf("cannot mount using driver %s in rootless mode", driver)
		}

		became, ret, err := rootless.BecomeRootInUserNS("")
		if err != nil {
			return nil, err
		}
		if became {
			os.Exit(ret)
		}
	}
	if opts.All {
		allImages, err := ir.Libpod.ImageRuntime().GetImages()
		if err != nil {
			return nil, err
		}
		for _, img := range allImages {
			if !img.IsReadOnly() {
				images = append(images, img)
			}
		}
	} else {
		for _, i := range nameOrIDs {
			img, err := ir.Libpod.ImageRuntime().NewFromLocal(i)
			if err != nil {
				return nil, err
			}
			images = append(images, img)
		}
	}
	reports := make([]*entities.ImageMountReport, 0, len(images))
	for _, img := range images {
		report := entities.ImageMountReport{Id: img.ID()}
		if img.IsReadOnly() {
			report.Err = errors.Errorf("mounting readonly %s image not supported", img.ID())
		} else {
			report.Path, report.Err = img.Mount([]string{}, "")
		}
		reports = append(reports, &report)
	}
	if len(reports) > 0 {
		return reports, nil
	}

	images, err = ir.Libpod.ImageRuntime().GetImages()
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		mounted, path, err := i.Mounted()
		if err != nil {
			if errors.Cause(err) == storage.ErrLayerUnknown {
				continue
			}
			return nil, err
		}
		if mounted {
			tags, err := i.RepoTags()
			if err != nil {
				return nil, err
			}
			reports = append(reports, &entities.ImageMountReport{
				Id:           i.ID(),
				Name:         string(i.Digest()),
				Repositories: tags,
				Path:         path,
			})
		}
	}
	return reports, nil
}

func (ir *ImageEngine) Unmount(ctx context.Context, nameOrIDs []string, options entities.ImageUnmountOptions) ([]*entities.ImageUnmountReport, error) {
	var images []*image.Image

	if options.All {
		allImages, err := ir.Libpod.ImageRuntime().GetImages()
		if err != nil {
			return nil, err
		}
		for _, img := range allImages {
			if !img.IsReadOnly() {
				images = append(images, img)
			}
		}
	} else {
		for _, i := range nameOrIDs {
			img, err := ir.Libpod.ImageRuntime().NewFromLocal(i)
			if err != nil {
				return nil, err
			}
			images = append(images, img)
		}
	}

	reports := []*entities.ImageUnmountReport{}
	for _, img := range images {
		report := entities.ImageUnmountReport{Id: img.ID()}
		mounted, _, err := img.Mounted()
		if err != nil {
			// Errors will be caught in Unmount call below
			// Default assumption to mounted
			mounted = true
		}
		if !mounted {
			continue
		}
		if err := img.Unmount(options.Force); err != nil {
			if options.All && errors.Cause(err) == storage.ErrLayerNotMounted {
				logrus.Debugf("Error umounting image %s, storage.ErrLayerNotMounted", img.ID())
				continue
			}
			report.Err = errors.Wrapf(err, "error unmounting image %s", img.ID())
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func ToDomainHistoryLayer(layer *libpodImage.History) entities.ImageHistoryLayer {
	l := entities.ImageHistoryLayer{}
	l.ID = layer.ID
	l.Created = *layer.Created
	l.CreatedBy = layer.CreatedBy
	copy(l.Tags, layer.Tags)
	l.Size = layer.Size
	l.Comment = layer.Comment
	return l
}

func pull(ctx context.Context, runtime *image.Runtime, rawImage string, options entities.ImagePullOptions, label *string) (*entities.ImagePullReport, error) {
	var writer io.Writer
	if !options.Quiet {
		writer = os.Stderr
	}

	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	imageRef, err := alltransports.ParseImageName(rawImage)
	if err != nil {
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", dockerPrefix, rawImage))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid image reference %q", rawImage)
		}
	}

	var registryCreds *types.DockerAuthConfig
	if len(options.Username) > 0 && len(options.Password) > 0 {
		registryCreds = &types.DockerAuthConfig{
			Username: options.Username,
			Password: options.Password,
		}
	}
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              options.CertDir,
		OSChoice:                    options.OverrideOS,
		ArchitectureChoice:          options.OverrideArch,
		VariantChoice:               options.OverrideVariant,
		DockerInsecureSkipTLSVerify: options.SkipTLSVerify,
	}

	if !options.AllTags {
		newImage, err := runtime.New(ctx, rawImage, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, label, options.PullPolicy)
		if err != nil {
			return nil, err
		}
		return &entities.ImagePullReport{Images: []string{newImage.ID()}}, nil
	}

	// --all-tags requires the docker transport
	if imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, errors.New("--all-tags requires docker transport")
	}

	// Trim the docker-transport prefix.
	rawImage = strings.TrimPrefix(rawImage, docker.Transport.Name())

	// all-tags doesn't work with a tagged reference, so let's check early
	namedRef, err := reference.Parse(rawImage)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing %q", rawImage)
	}
	if _, isTagged := namedRef.(reference.Tagged); isTagged {
		return nil, errors.New("--all-tags requires a reference without a tag")

	}

	systemContext := image.GetSystemContext("", options.Authfile, false)
	tags, err := docker.GetRepositoryTags(ctx, systemContext, imageRef)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting repository tags")
	}

	foundIDs := []string{}
	for _, tag := range tags {
		name := rawImage + ":" + tag
		newImage, err := runtime.New(ctx, name, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			logrus.Errorf("error pulling image %q", name)
			continue
		}
		foundIDs = append(foundIDs, newImage.ID())
	}

	if len(tags) != len(foundIDs) {
		return nil, err
	}
	return &entities.ImagePullReport{Images: foundIDs}, nil
}

func (ir *ImageEngine) Pull(ctx context.Context, rawImage string, options entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	return pull(ctx, ir.Libpod.ImageRuntime(), rawImage, options, nil)
}

func (ir *ImageEngine) Inspect(ctx context.Context, namesOrIDs []string, opts entities.InspectOptions) ([]*entities.ImageInspectReport, []error, error) {
	reports := []*entities.ImageInspectReport{}
	errs := []error{}
	for _, i := range namesOrIDs {
		img, err := ir.Libpod.ImageRuntime().NewFromLocal(i)
		if err != nil {
			// This is probably a no such image, treat as nonfatal.
			errs = append(errs, err)
			continue
		}
		result, err := img.Inspect(ctx)
		if err != nil {
			// This is more likely to be fatal.
			return nil, nil, err
		}
		report := entities.ImageInspectReport{}
		if err := domainUtils.DeepCopy(&report, result); err != nil {
			return nil, nil, err
		}
		reports = append(reports, &report)
	}
	return reports, errs, nil
}

func (ir *ImageEngine) Push(ctx context.Context, source string, destination string, options entities.ImagePushOptions) error {
	var writer io.Writer
	if !options.Quiet {
		writer = os.Stderr
	}

	var manifestType string
	switch options.Format {
	case "":
		// Default
	case "oci":
		manifestType = imgspecv1.MediaTypeImageManifest
	case "v2s1":
		manifestType = manifest.DockerV2Schema1SignedMediaType
	case "v2s2", "docker":
		manifestType = manifest.DockerV2Schema2MediaType
	default:
		return errors.Errorf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", options.Format)
	}

	var registryCreds *types.DockerAuthConfig
	if len(options.Username) > 0 && len(options.Password) > 0 {
		registryCreds = &types.DockerAuthConfig{
			Username: options.Username,
			Password: options.Password,
		}
	}
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              options.CertDir,
		DockerInsecureSkipTLSVerify: options.SkipTLSVerify,
	}

	signOptions := image.SigningOptions{
		RemoveSignatures: options.RemoveSignatures,
		SignBy:           options.SignBy,
	}

	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(source)
	if err != nil {
		return err
	}

	return newImage.PushImageToHeuristicDestination(
		ctx,
		destination,
		manifestType,
		options.Authfile,
		options.DigestFile,
		options.SignaturePolicy,
		writer,
		options.Compress,
		signOptions,
		&dockerRegistryOptions,
		nil)
}

// func (r *imageRuntime) Delete(ctx context.Context, nameOrID string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
// 	image, err := r.libpod.ImageEngine().NewFromLocal(nameOrID)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	results, err := r.libpod.RemoveImage(ctx, image, opts.Force)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	report := entities.ImageDeleteReport{}
// 	if err := domainUtils.DeepCopy(&report, results); err != nil {
// 		return nil, err
// 	}
// 	return &report, nil
// }
//
// func (r *imageRuntime) Prune(ctx context.Context, opts entities.ImagePruneOptions) (*entities.ImagePruneReport, error) {
// 	// TODO: map FilterOptions
// 	id, err := r.libpod.ImageEngine().PruneImages(ctx, opts.All, []string{})
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	// TODO: Determine Size
// 	report := entities.ImagePruneReport{}
// 	copy(report.Report.ID, id)
// 	return &report, nil
// }

func (ir *ImageEngine) Tag(ctx context.Context, nameOrID string, tags []string, options entities.ImageTagOptions) error {
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if err := newImage.TagImage(tag); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(ctx context.Context, nameOrID string, tags []string, options entities.ImageUntagOptions) error {
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return err
	}
	// If only one arg is provided, all names are to be untagged
	if len(tags) == 0 {
		tags = newImage.Names()
	}
	for _, tag := range tags {
		if err := newImage.UntagImage(tag); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Load(ctx context.Context, opts entities.ImageLoadOptions) (*entities.ImageLoadReport, error) {
	var (
		writer io.Writer
	)
	if !opts.Quiet {
		writer = os.Stderr
	}
	name, err := ir.Libpod.LoadImage(ctx, opts.Name, opts.Input, writer, opts.SignaturePolicy)
	if err != nil {
		return nil, err
	}
	names := strings.Split(name, ",")
	if len(names) <= 1 {
		newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(name)
		if err != nil {
			return nil, errors.Wrap(err, "image loaded but no additional tags were created")
		}
		if len(opts.Name) > 0 {
			if err := newImage.TagImage(fmt.Sprintf("%s:%s", opts.Name, opts.Tag)); err != nil {
				return nil, errors.Wrapf(err, "error adding %q to image %q", opts.Name, newImage.InputName)
			}
		}
	}
	return &entities.ImageLoadReport{Names: names}, nil
}

func (ir *ImageEngine) Import(ctx context.Context, opts entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	id, err := ir.Libpod.Import(ctx, opts.Source, opts.Reference, opts.SignaturePolicy, opts.Changes, opts.Message, opts.Quiet)
	if err != nil {
		return nil, err
	}
	return &entities.ImageImportReport{Id: id}, nil
}

func (ir *ImageEngine) Save(ctx context.Context, nameOrID string, tags []string, options entities.ImageSaveOptions) error {
	if options.MultiImageArchive {
		nameOrIDs := append([]string{nameOrID}, tags...)
		return ir.Libpod.ImageRuntime().SaveImages(ctx, nameOrIDs, options.Format, options.Output, options.Quiet, true)
	}
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return err
	}
	return newImage.Save(ctx, nameOrID, options.Format, options.Output, tags, options.Quiet, options.Compress, true)
}

func (ir *ImageEngine) Diff(_ context.Context, nameOrID string, _ entities.DiffOptions) (*entities.DiffReport, error) {
	changes, err := ir.Libpod.GetDiff("", nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.DiffReport{Changes: changes}, nil
}

func (ir *ImageEngine) Search(ctx context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	filter, err := image.ParseSearchFilter(opts.Filters)
	if err != nil {
		return nil, err
	}

	searchOpts := image.SearchOptions{
		Authfile:              opts.Authfile,
		Filter:                *filter,
		Limit:                 opts.Limit,
		NoTrunc:               opts.NoTrunc,
		InsecureSkipTLSVerify: opts.SkipTLSVerify,
		ListTags:              opts.ListTags,
	}

	searchResults, err := image.SearchImages(term, searchOpts)
	if err != nil {
		return nil, err
	}

	// Convert from image.SearchResults to entities.ImageSearchReport. We don't
	// want to leak any low-level packages into the remote client, which
	// requires converting.
	reports := make([]entities.ImageSearchReport, len(searchResults))
	for i := range searchResults {
		reports[i].Index = searchResults[i].Index
		reports[i].Name = searchResults[i].Name
		reports[i].Description = searchResults[i].Description
		reports[i].Stars = searchResults[i].Stars
		reports[i].Official = searchResults[i].Official
		reports[i].Automated = searchResults[i].Automated
		reports[i].Tag = searchResults[i].Tag
	}

	return reports, nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (ir *ImageEngine) Config(_ context.Context) (*config.Config, error) {
	return ir.Libpod.GetConfig()
}

func (ir *ImageEngine) Build(ctx context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {

	id, _, err := ir.Libpod.Build(ctx, opts.BuildOptions, containerFiles...)
	if err != nil {
		return nil, err
	}
	return &entities.BuildReport{ID: id}, nil
}

func (ir *ImageEngine) Tree(ctx context.Context, nameOrID string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	img, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return nil, err
	}
	results, err := img.GenerateTree(opts.WhatRequires)
	if err != nil {
		return nil, err
	}
	return &entities.ImageTreeReport{Tree: results}, nil
}

// removeErrorsToExitCode returns an exit code for the specified slice of
// image-removal errors. The error codes are set according to the documented
// behaviour in the Podman man pages.
func removeErrorsToExitCode(rmErrors []error) int {
	var (
		// noSuchImageErrors indicates that at least one image was not found.
		noSuchImageErrors bool
		// inUseErrors indicates that at least one image is being used by a
		// container.
		inUseErrors bool
		// otherErrors indicates that at least one error other than the two
		// above occurred.
		otherErrors bool
	)

	if len(rmErrors) == 0 {
		return 0
	}

	for _, e := range rmErrors {
		switch errors.Cause(e) {
		case define.ErrNoSuchImage:
			noSuchImageErrors = true
		case define.ErrImageInUse, storage.ErrImageUsedByContainer:
			inUseErrors = true
		default:
			otherErrors = true
		}
	}

	switch {
	case inUseErrors:
		// One of the specified images has child images or is
		// being used by a container.
		return 2
	case noSuchImageErrors && !(otherErrors || inUseErrors):
		// One of the specified images did not exist, and no other
		// failures.
		return 1
	default:
		return 125
	}
}

// Remove removes one or more images from local storage.
func (ir *ImageEngine) Remove(ctx context.Context, images []string, opts entities.ImageRemoveOptions) (report *entities.ImageRemoveReport, rmErrors []error) {
	report = &entities.ImageRemoveReport{}

	// Set the exit code at very end.
	defer func() {
		report.ExitCode = removeErrorsToExitCode(rmErrors)
	}()

	// deleteImage is an anonymous function to conveniently delete an image
	// without having to pass all local data around.
	deleteImage := func(img *image.Image) error {
		results, err := ir.Libpod.RemoveImage(ctx, img, opts.Force)
		if err != nil {
			return err
		}
		report.Deleted = append(report.Deleted, results.Deleted)
		report.Untagged = append(report.Untagged, results.Untagged...)
		return nil
	}

	// Delete all images from the local storage.
	if opts.All {
		previousImages := 0
		// Remove all images one-by-one.
		for {
			storageImages, err := ir.Libpod.ImageRuntime().GetRWImages()
			if err != nil {
				rmErrors = append(rmErrors, err)
				return
			}
			// No images (left) to remove, so we're done.
			if len(storageImages) == 0 {
				return
			}
			// Prevent infinity loops by making a delete-progress check.
			if previousImages == len(storageImages) {
				rmErrors = append(rmErrors, errors.New("unable to delete all images, check errors and re-run image removal if needed"))
				break
			}
			previousImages = len(storageImages)
			// Delete all "leaves" (i.e., images without child images).
			for _, img := range storageImages {
				isParent, err := img.IsParent(ctx)
				if err != nil {
					rmErrors = append(rmErrors, err)
					continue
				}
				// Skip parent images.
				if isParent {
					continue
				}
				if err := deleteImage(img); err != nil {
					rmErrors = append(rmErrors, err)
				}
			}
		}

		return
	}

	// Delete only the specified images.
	for _, id := range images {
		img, err := ir.Libpod.ImageRuntime().NewFromLocal(id)
		if err != nil {
			rmErrors = append(rmErrors, err)
			continue
		}
		err = deleteImage(img)
		if err != nil {
			rmErrors = append(rmErrors, err)
		}
	}
	return //nolint
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
	shutdownSync.Do(func() {
		_ = ir.Libpod.Shutdown(false)
	})
}

func (ir *ImageEngine) Sign(ctx context.Context, names []string, options entities.SignOptions) (*entities.SignReport, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, errors.Wrap(err, "error initializing GPG")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, errors.Wrap(err, "signing is not supported")
	}
	sc := ir.Libpod.SystemContext()
	sc.DockerCertPath = options.CertDir

	systemRegistriesDirPath := trust.RegistriesDirPath(sc)
	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading registry configuration")
	}

	for _, signimage := range names {
		err = func() error {
			srcRef, err := alltransports.ParseImageName(signimage)
			if err != nil {
				return errors.Wrapf(err, "error parsing image name")
			}
			rawSource, err := srcRef.NewImageSource(ctx, sc)
			if err != nil {
				return errors.Wrapf(err, "error getting image source")
			}
			defer func() {
				if err = rawSource.Close(); err != nil {
					logrus.Errorf("unable to close %s image source %q", srcRef.DockerReference().Name(), err)
				}
			}()
			getManifest, _, err := rawSource.GetManifest(ctx, nil)
			if err != nil {
				return errors.Wrapf(err, "error getting getManifest")
			}
			dockerReference := rawSource.Reference().DockerReference()
			if dockerReference == nil {
				return errors.Errorf("cannot determine canonical Docker reference for destination %s", transports.ImageName(rawSource.Reference()))
			}
			var sigStoreDir string
			if options.Directory != "" {
				sigStoreDir = options.Directory
			}
			if sigStoreDir == "" {
				if rootless.IsRootless() {
					sigStoreDir = filepath.Join(filepath.Dir(ir.Libpod.StorageConfig().GraphRoot), "sigstore")
				} else {
					var sigStoreURI string
					registryInfo := trust.HaveMatchRegistry(rawSource.Reference().DockerReference().String(), registryConfigs)
					if registryInfo != nil {
						if sigStoreURI = registryInfo.SigStoreStaging; sigStoreURI == "" {
							sigStoreURI = registryInfo.SigStore
						}
					}
					if sigStoreURI == "" {
						return errors.Errorf("no signature storage configuration found for %s", rawSource.Reference().DockerReference().String())

					}
					sigStoreDir, err = localPathFromURI(sigStoreURI)
					if err != nil {
						return errors.Wrapf(err, "invalid signature storage %s", sigStoreURI)
					}
				}
			}
			manifestDigest, err := manifest.Digest(getManifest)
			if err != nil {
				return err
			}
			repo := reference.Path(dockerReference)
			if path.Clean(repo) != repo { // Coverage: This should not be reachable because /./ and /../ components are not valid in docker references
				return errors.Errorf("Unexpected path elements in Docker reference %s for signature storage", dockerReference.String())
			}

			// create signature
			newSig, err := signature.SignDockerManifest(getManifest, dockerReference.String(), mech, options.SignBy)
			if err != nil {
				return errors.Wrapf(err, "error creating new signature")
			}
			// create the signstore file
			signatureDir := fmt.Sprintf("%s@%s=%s", filepath.Join(sigStoreDir, repo), manifestDigest.Algorithm(), manifestDigest.Hex())
			if err := os.MkdirAll(signatureDir, 0751); err != nil {
				// The directory is allowed to exist
				if !os.IsExist(err) {
					logrus.Error(err)
					return nil
				}
			}
			sigFilename, err := getSigFilename(signatureDir)
			if err != nil {
				logrus.Errorf("error creating sigstore file: %v", err)
				return nil
			}
			err = ioutil.WriteFile(filepath.Join(signatureDir, sigFilename), newSig, 0644)
			if err != nil {
				logrus.Errorf("error storing signature for %s", rawSource.Reference().DockerReference().String())
				return nil
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func getSigFilename(sigStoreDirPath string) (string, error) {
	sigFileSuffix := 1
	sigFiles, err := ioutil.ReadDir(sigStoreDirPath)
	if err != nil {
		return "", err
	}
	sigFilenames := make(map[string]bool)
	for _, file := range sigFiles {
		sigFilenames[file.Name()] = true
	}
	for {
		sigFilename := "signature-" + strconv.Itoa(sigFileSuffix)
		if _, exists := sigFilenames[sigFilename]; !exists {
			return sigFilename, nil
		}
		sigFileSuffix++
	}
}

func localPathFromURI(sigStoreDir string) (string, error) {
	url, err := url.Parse(sigStoreDir)
	if err != nil {
		return sigStoreDir, errors.Wrapf(err, "invalid directory %s", sigStoreDir)
	}
	if url.Scheme != "file" {
		return sigStoreDir, errors.Errorf("writing to %s is not supported. Use a supported scheme", sigStoreDir)
	}
	sigStoreDir = url.Path
	return sigStoreDir, nil
}
