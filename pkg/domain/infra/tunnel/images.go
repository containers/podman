package tunnel

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	images "github.com/containers/podman/v2/pkg/bindings/images"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/utils"
	utils2 "github.com/containers/podman/v2/utils"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	found, err := images.Exists(ir.ClientCxt, nameOrID)
	return &entities.BoolReport{Value: found}, err
}

func (ir *ImageEngine) Remove(ctx context.Context, imagesArg []string, opts entities.ImageRemoveOptions) (*entities.ImageRemoveReport, []error) {
	return images.BatchRemove(ir.ClientCxt, imagesArg, opts)
}

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {

	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.Split(filter, "=")
		filters[f[0]] = f[1:]
	}
	images, err := images.List(ir.ClientCxt, &opts.All, filters)
	if err != nil {
		return nil, err
	}

	is := make([]*entities.ImageSummary, len(images))
	for i, img := range images {
		hold := entities.ImageSummary{}
		if err := utils.DeepCopy(&hold, img); err != nil {
			return nil, err
		}
		is[i] = &hold
	}
	return is, nil
}

func (ir *ImageEngine) Mount(ctx context.Context, images []string, options entities.ImageMountOptions) ([]*entities.ImageMountReport, error) {
	return nil, errors.New("mounting images is not supported for remote clients")
}

func (ir *ImageEngine) Unmount(ctx context.Context, images []string, options entities.ImageUnmountOptions) ([]*entities.ImageUnmountReport, error) {
	return nil, errors.New("unmounting images is not supported for remote clients")
}

func (ir *ImageEngine) History(ctx context.Context, nameOrID string, opts entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	results, err := images.History(ir.ClientCxt, nameOrID)
	if err != nil {
		return nil, err
	}

	history := entities.ImageHistoryReport{
		Layers: make([]entities.ImageHistoryLayer, len(results)),
	}

	for i, layer := range results {
		// Created time comes over as an int64 so needs conversion to time.time
		t := time.Unix(layer.Created, 0)
		hold := entities.ImageHistoryLayer{
			ID:        layer.ID,
			Created:   t.UTC(),
			CreatedBy: layer.CreatedBy,
			Tags:      layer.Tags,
			Size:      layer.Size,
			Comment:   layer.Comment,
		}
		history.Layers[i] = hold
	}
	return &history, nil
}

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) (*entities.ImagePruneReport, error) {
	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.Split(filter, "=")
		filters[f[0]] = f[1:]
	}

	results, err := images.Prune(ir.ClientCxt, &opts.All, filters)
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

func (ir *ImageEngine) Pull(ctx context.Context, rawImage string, options entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	pulledImages, err := images.Pull(ir.ClientCxt, rawImage, options)
	if err != nil {
		return nil, err
	}
	return &entities.ImagePullReport{Images: pulledImages}, nil
}

func (ir *ImageEngine) Tag(ctx context.Context, nameOrID string, tags []string, options entities.ImageTagOptions) error {
	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return errors.Wrapf(err, "error parsing reference %q", newTag)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return errors.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Tag(ir.ClientCxt, nameOrID, tag, repo); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(ctx context.Context, nameOrID string, tags []string, options entities.ImageUntagOptions) error {
	if len(tags) == 0 {
		return images.Untag(ir.ClientCxt, nameOrID, "", "")
	}

	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return errors.Wrapf(err, "error parsing reference %q", newTag)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return errors.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Untag(ir.ClientCxt, nameOrID, tag, repo); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Inspect(ctx context.Context, namesOrIDs []string, opts entities.InspectOptions) ([]*entities.ImageInspectReport, []error, error) {
	reports := []*entities.ImageInspectReport{}
	errs := []error{}
	for _, i := range namesOrIDs {
		r, err := images.GetImage(ir.ClientCxt, i, &opts.Size)
		if err != nil {
			errModel, ok := err.(entities.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, errors.Wrapf(err, "unable to inspect %q", i))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, r)
	}
	return reports, errs, nil
}

func (ir *ImageEngine) Load(ctx context.Context, opts entities.ImageLoadOptions) (*entities.ImageLoadReport, error) {
	f, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fInfo.IsDir() {
		return nil, errors.Errorf("remote client supports archives only but %q is a directory", opts.Input)
	}
	ref := opts.Name
	if len(opts.Tag) > 0 {
		ref += ":" + opts.Tag
	}
	return images.Load(ir.ClientCxt, f, &ref)
}

func (ir *ImageEngine) Import(ctx context.Context, opts entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	var (
		err       error
		sourceURL *string
		f         *os.File
	)
	if opts.SourceIsURL {
		sourceURL = &opts.Source
	} else {
		f, err = os.Open(opts.Source)
		if err != nil {
			return nil, err
		}
	}
	return images.Import(ir.ClientCxt, opts.Changes, &opts.Message, &opts.Reference, sourceURL, f)
}

func (ir *ImageEngine) Push(ctx context.Context, source string, destination string, options entities.ImagePushOptions) error {
	return images.Push(ir.ClientCxt, source, destination, options)
}

func (ir *ImageEngine) Save(ctx context.Context, nameOrID string, tags []string, options entities.ImageSaveOptions) error {
	var (
		f   *os.File
		err error
	)
	switch options.Format {
	case "oci-dir", "docker-dir":
		f, err = ioutil.TempFile("", "podman_save")
		if err == nil {
			defer func() { _ = os.Remove(f.Name()) }()
		}
	default:
		f, err = os.Create(options.Output)
	}
	if err != nil {
		return err
	}

	if options.MultiImageArchive {
		exErr := images.MultiExport(ir.ClientCxt, append([]string{nameOrID}, tags...), f, &options.Format, &options.Compress)
		if err := f.Close(); err != nil {
			return err
		}
		if exErr != nil {
			return exErr
		}
	} else {
		// FIXME: tags are entirely ignored here but shouldn't.
		exErr := images.Export(ir.ClientCxt, nameOrID, f, &options.Format, &options.Compress)
		if err := f.Close(); err != nil {
			return err
		}
		if exErr != nil {
			return exErr
		}
	}

	if options.Format != "oci-dir" && options.Format != "docker-dir" {
		return nil
	}

	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}
	info, err := os.Stat(options.Output)
	switch {
	case err == nil:
		if info.Mode().IsRegular() {
			return errors.Errorf("%q already exists as a regular file", options.Output)
		}
	case os.IsNotExist(err):
		if err := os.Mkdir(options.Output, 0755); err != nil {
			return err
		}
	default:
		return err
	}
	return utils2.UntarToFileSystem(options.Output, f, nil)
}

// Diff reports the changes to the given image
func (ir *ImageEngine) Diff(ctx context.Context, nameOrID string, _ entities.DiffOptions) (*entities.DiffReport, error) {
	changes, err := images.Diff(ir.ClientCxt, nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.DiffReport{Changes: changes}, nil
}

func (ir *ImageEngine) Search(ctx context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	return images.Search(ir.ClientCxt, term, opts)
}

func (ir *ImageEngine) Config(_ context.Context) (*config.Config, error) {
	return config.Default()
}

func (ir *ImageEngine) Build(_ context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	report, err := images.Build(ir.ClientCxt, containerFiles, opts)
	if err != nil {
		return nil, err
	}
	// For remote clients, if the option for writing to a file was
	// selected, we need to write to the *client's* filesystem.
	if len(opts.IIDFile) > 0 {
		f, err := os.Create(opts.IIDFile)
		if err != nil {
			return nil, err
		}
		if _, err := f.WriteString(report.ID); err != nil {
			return nil, err
		}
	}
	return report, nil
}

func (ir *ImageEngine) Tree(ctx context.Context, nameOrID string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	return images.Tree(ir.ClientCxt, nameOrID, &opts.WhatRequires)
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
}

func (ir *ImageEngine) Sign(ctx context.Context, names []string, options entities.SignOptions) (*entities.SignReport, error) {
	return nil, errors.New("not implemented yet")
}
