package tunnel

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/libpod/pkg/bindings"
	images "github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/utils"
	utils2 "github.com/containers/libpod/utils"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrId string) (*entities.BoolReport, error) {
	found, err := images.Exists(ir.ClientCxt, nameOrId)
	return &entities.BoolReport{Value: found}, err
}

func (ir *ImageEngine) Remove(ctx context.Context, imagesArg []string, opts entities.ImageRemoveOptions) (*entities.ImageRemoveReport, []error) {
	return images.BatchRemove(ir.ClientCxt, imagesArg, opts)
}

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	images, err := images.List(ir.ClientCxt, &opts.All, opts.Filters)

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

func (ir *ImageEngine) History(ctx context.Context, nameOrId string, opts entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	results, err := images.History(ir.ClientCxt, nameOrId)
	if err != nil {
		return nil, err
	}

	history := entities.ImageHistoryReport{
		Layers: make([]entities.ImageHistoryLayer, len(results)),
	}

	for i, layer := range results {
		hold := entities.ImageHistoryLayer{}
		_ = utils.DeepCopy(&hold, layer)
		history.Layers[i] = hold
	}
	return &history, nil
}

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) (*entities.ImagePruneReport, error) {
	results, err := images.Prune(ir.ClientCxt, &opts.All, opts.Filters)
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

func (ir *ImageEngine) Tag(ctx context.Context, nameOrId string, tags []string, options entities.ImageTagOptions) error {
	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return err
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return errors.Errorf("invalid image name %q", nameOrId)
		}
		if err := images.Tag(ir.ClientCxt, nameOrId, tag, repo); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(ctx context.Context, nameOrId string, tags []string, options entities.ImageUntagOptions) error {
	// Remove all tags if none are provided
	if len(tags) == 0 {
		newImage, err := images.GetImage(ir.ClientCxt, nameOrId, bindings.PFalse)
		if err != nil {
			return err
		}
		tags = newImage.NamesHistory
	}

	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return err
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return errors.Errorf("invalid image name %q", nameOrId)
		}
		if err := images.Untag(ir.ClientCxt, nameOrId, tag, repo); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Inspect(ctx context.Context, namesOrIDs []string, opts entities.InspectOptions) ([]*entities.ImageInspectReport, error) {
	reports := []*entities.ImageInspectReport{}
	for _, i := range namesOrIDs {
		r, err := images.GetImage(ir.ClientCxt, i, &opts.Size)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}

func (ir *ImageEngine) Load(ctx context.Context, opts entities.ImageLoadOptions) (*entities.ImageLoadReport, error) {
	f, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return images.Load(ir.ClientCxt, f, &opts.Name)
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

func (ir *ImageEngine) Save(ctx context.Context, nameOrId string, tags []string, options entities.ImageSaveOptions) error {
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

	exErr := images.Export(ir.ClientCxt, nameOrId, f, &options.Format, &options.Compress)
	if err := f.Close(); err != nil {
		return err
	}
	if exErr != nil {
		return exErr
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
func (ir *ImageEngine) Diff(ctx context.Context, nameOrId string, _ entities.DiffOptions) (*entities.DiffReport, error) {
	changes, err := images.Diff(ir.ClientCxt, nameOrId)
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

func (ir *ImageEngine) Build(ctx context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	return nil, errors.New("not implemented yet")
}

func (ir *ImageEngine) Tree(ctx context.Context, nameOrId string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	return images.Tree(ir.ClientCxt, nameOrId, &opts.WhatRequires)
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
}

func (ir *ImageEngine) Sign(ctx context.Context, names []string, options entities.SignOptions) (*entities.SignReport, error) {
	return nil, errors.New("not implemented yet")
}
