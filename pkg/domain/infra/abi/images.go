// +build ABISupport

package abi

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/v5/docker"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	domainUtils "github.com/containers/libpod/pkg/domain/utils"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrId string) (*entities.BoolReport, error) {
	_, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
	if err != nil && errors.Cause(err) != define.ErrNoSuchImage {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ir *ImageEngine) Delete(ctx context.Context, nameOrId []string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
	report := entities.ImageDeleteReport{}

	if opts.All {
		var previousTargets []*libpodImage.Image
	repeatRun:
		targets, err := ir.Libpod.ImageRuntime().GetRWImages()
		if err != nil {
			return &report, errors.Wrapf(err, "unable to query local images")
		}

		if len(targets) > 0 && len(targets) == len(previousTargets) {
			return &report, errors.New("unable to delete all images; re-run the rmi command again.")
		}
		previousTargets = targets

		for _, img := range targets {
			isParent, err := img.IsParent(ctx)
			if err != nil {
				return &report, err
			}
			if isParent {
				continue
			}
			err = ir.deleteImage(ctx, img, opts, report)
			report.Errors = append(report.Errors, err)
		}
		if len(previousTargets) != 1 {
			goto repeatRun
		}
		return &report, nil
	}

	for _, id := range nameOrId {
		image, err := ir.Libpod.ImageRuntime().NewFromLocal(id)
		if err != nil {
			return nil, err
		}

		err = ir.deleteImage(ctx, image, opts, report)
		if err != nil {
			return &report, err
		}
	}
	return &report, nil
}

func (ir *ImageEngine) deleteImage(ctx context.Context, img *libpodImage.Image, opts entities.ImageDeleteOptions, report entities.ImageDeleteReport) error {
	results, err := ir.Libpod.RemoveImage(ctx, img, opts.Force)
	switch errors.Cause(err) {
	case nil:
		break
	case storage.ErrImageUsedByContainer:
		report.ImageInUse = errors.New(
			fmt.Sprintf("A container associated with containers/storage, i.e. via Buildah, CRI-O, etc., may be associated with this image: %-12.12s\n", img.ID()))
		return nil
	case libpodImage.ErrNoSuchImage:
		report.ImageNotFound = err
		return nil
	default:
		return err
	}

	report.Deleted = append(report.Deleted, results.Deleted)
	report.Untagged = append(report.Untagged, results.Untagged...)
	return nil
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
		Size: 0,
	}
	return &report, nil
}

func (ir *ImageEngine) History(ctx context.Context, nameOrId string, opts entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	image, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
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

func ToDomainHistoryLayer(layer *libpodImage.History) entities.ImageHistoryLayer {
	l := entities.ImageHistoryLayer{}
	l.ID = layer.ID
	l.Created = layer.Created.Unix()
	l.CreatedBy = layer.CreatedBy
	copy(l.Tags, layer.Tags)
	l.Size = layer.Size
	l.Comment = layer.Comment
	return l
}

func (ir *ImageEngine) Pull(ctx context.Context, rawImage string, options entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	var writer io.Writer
	if !options.Quiet {
		writer = os.Stderr
	}

	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	imageRef, err := alltransports.ParseImageName(rawImage)
	if err != nil {
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", dockerPrefix, rawImage))
		if err != nil {
			return nil, errors.Errorf("invalid image reference %q", rawImage)
		}
	}

	// Special-case for docker-archive which allows multiple tags.
	if imageRef.Transport().Name() == dockerarchive.Transport.Name() {
		newImage, err := ir.Libpod.ImageRuntime().LoadFromArchiveReference(ctx, imageRef, options.SignaturePolicy, writer)
		if err != nil {
			return nil, errors.Wrapf(err, "error pulling image %q", rawImage)
		}
		return &entities.ImagePullReport{Images: []string{newImage[0].ID()}}, nil
	}

	var registryCreds *types.DockerAuthConfig
	if options.Credentials != "" {
		creds, err := util.ParseRegistryCreds(options.Credentials)
		if err != nil {
			return nil, err
		}
		registryCreds = creds
	}
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              options.CertDir,
		OSChoice:                    options.OverrideOS,
		ArchitectureChoice:          options.OverrideArch,
		DockerInsecureSkipTLSVerify: options.TLSVerify,
	}

	if !options.AllTags {
		newImage, err := ir.Libpod.ImageRuntime().New(ctx, rawImage, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			return nil, errors.Wrapf(err, "error pulling image %q", rawImage)
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

	var foundIDs []string
	for _, tag := range tags {
		name := rawImage + ":" + tag
		newImage, err := ir.Libpod.ImageRuntime().New(ctx, name, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageAlways)
		if err != nil {
			logrus.Errorf("error pulling image %q", name)
			continue
		}
		foundIDs = append(foundIDs, newImage.ID())
	}

	if len(tags) != len(foundIDs) {
		return nil, errors.Errorf("error pulling image %q", rawImage)
	}
	return &entities.ImagePullReport{Images: foundIDs}, nil
}

func (ir *ImageEngine) Inspect(ctx context.Context, names []string, opts entities.InspectOptions) (*entities.ImageInspectReport, error) {
	report := entities.ImageInspectReport{
		Errors: make(map[string]error),
	}

	for _, id := range names {
		img, err := ir.Libpod.ImageRuntime().NewFromLocal(id)
		if err != nil {
			report.Errors[id] = err
			continue
		}

		results, err := img.Inspect(ctx)
		if err != nil {
			report.Errors[id] = err
			continue
		}

		cookedResults := entities.ImageData{}
		_ = domainUtils.DeepCopy(&cookedResults, results)
		report.Images = append(report.Images, &cookedResults)
	}
	return &report, nil
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
		return fmt.Errorf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", options.Format)
	}

	var registryCreds *types.DockerAuthConfig
	if options.Credentials != "" {
		creds, err := util.ParseRegistryCreds(options.Credentials)
		if err != nil {
			return err
		}
		registryCreds = creds
	}
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              options.CertDir,
		DockerInsecureSkipTLSVerify: options.TLSVerify,
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

// func (r *imageRuntime) Delete(ctx context.Context, nameOrId string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
// 	image, err := r.libpod.ImageEngine().NewFromLocal(nameOrId)
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
// 	copy(report.Report.Id, id)
// 	return &report, nil
// }

func (ir *ImageEngine) Tag(ctx context.Context, nameOrId string, tags []string, options entities.ImageTagOptions) error {
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
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

func (ir *ImageEngine) Untag(ctx context.Context, nameOrId string, tags []string, options entities.ImageUntagOptions) error {
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
	if err != nil {
		return err
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
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return nil, errors.Wrap(err, "image loaded but no additional tags were created")
	}
	if err := newImage.TagImage(opts.Name); err != nil {
		return nil, errors.Wrapf(err, "error adding %q to image %q", opts.Name, newImage.InputName)
	}
	return &entities.ImageLoadReport{Name: name}, nil
}

func (ir *ImageEngine) Import(ctx context.Context, opts entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	id, err := ir.Libpod.Import(ctx, opts.Source, opts.Reference, opts.Changes, opts.Message, opts.Quiet)
	if err != nil {
		return nil, err
	}
	return &entities.ImageImportReport{Id: id}, nil
}

func (ir *ImageEngine) Save(ctx context.Context, nameOrId string, tags []string, options entities.ImageSaveOptions) error {
	newImage, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
	if err != nil {
		return err
	}
	return newImage.Save(ctx, nameOrId, options.Format, options.Output, tags, options.Quiet, options.Compress)
}

func (ir *ImageEngine) Diff(_ context.Context, nameOrId string, _ entities.DiffOptions) (*entities.DiffReport, error) {
	changes, err := ir.Libpod.GetDiff("", nameOrId)
	if err != nil {
		return nil, err
	}
	return &entities.DiffReport{Changes: changes}, nil
}
