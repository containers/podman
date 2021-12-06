package tunnel

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	images "github.com/containers/podman/v3/pkg/bindings/images"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	"github.com/containers/podman/v3/pkg/domain/utils"
	"github.com/containers/podman/v3/pkg/errorhandling"
	utils2 "github.com/containers/podman/v3/utils"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	found, err := images.Exists(ir.ClientCtx, nameOrID, nil)
	return &entities.BoolReport{Value: found}, err
}

func (ir *ImageEngine) Remove(ctx context.Context, imagesArg []string, opts entities.ImageRemoveOptions) (*entities.ImageRemoveReport, []error) {
	options := new(images.RemoveOptions).WithForce(opts.Force).WithAll(opts.All)
	return images.Remove(ir.ClientCtx, imagesArg, options)
}

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.Split(filter, "=")
		filters[f[0]] = f[1:]
	}
	options := new(images.ListOptions).WithAll(opts.All).WithFilters(filters)
	psImages, err := images.List(ir.ClientCtx, options)
	if err != nil {
		return nil, err
	}

	is := make([]*entities.ImageSummary, len(psImages))
	for i, img := range psImages {
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
	options := new(images.HistoryOptions)
	results, err := images.History(ir.ClientCtx, nameOrID, options)
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

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) ([]*reports.PruneReport, error) {
	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.Split(filter, "=")
		filters[f[0]] = f[1:]
	}
	options := new(images.PruneOptions).WithAll(opts.All).WithFilters(filters)
	reports, err := images.Prune(ir.ClientCtx, options)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func (ir *ImageEngine) Pull(ctx context.Context, rawImage string, opts entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	options := new(images.PullOptions)
	options.WithAllTags(opts.AllTags).WithAuthfile(opts.Authfile).WithArch(opts.Arch).WithOS(opts.OS)
	options.WithVariant(opts.Variant).WithPassword(opts.Password)
	options.WithQuiet(opts.Quiet).WithUsername(opts.Username).WithPolicy(opts.PullPolicy.String())
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	pulledImages, err := images.Pull(ir.ClientCtx, rawImage, options)
	if err != nil {
		return nil, err
	}
	return &entities.ImagePullReport{Images: pulledImages}, nil
}

func (ir *ImageEngine) Tag(ctx context.Context, nameOrID string, tags []string, opt entities.ImageTagOptions) error {
	options := new(images.TagOptions)
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
		if err := images.Tag(ir.ClientCtx, nameOrID, tag, repo, options); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(ctx context.Context, nameOrID string, tags []string, opt entities.ImageUntagOptions) error {
	options := new(images.UntagOptions)
	if len(tags) == 0 {
		return images.Untag(ir.ClientCtx, nameOrID, "", "", options)
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
		if t, ok := ref.(reference.Digested); ok {
			tag += "@" + t.Digest().String()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return errors.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Untag(ir.ClientCtx, nameOrID, tag, repo, options); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Inspect(ctx context.Context, namesOrIDs []string, opts entities.InspectOptions) ([]*entities.ImageInspectReport, []error, error) {
	options := new(images.GetOptions).WithSize(opts.Size)
	reports := []*entities.ImageInspectReport{}
	errs := []error{}
	for _, i := range namesOrIDs {
		r, err := images.GetImage(ir.ClientCtx, i, options)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
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
	return images.Load(ir.ClientCtx, f)
}

func (ir *ImageEngine) Import(ctx context.Context, opts entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	var (
		err error
		f   *os.File
	)
	options := new(images.ImportOptions).WithChanges(opts.Changes).WithMessage(opts.Message).WithReference(opts.Reference)
	if opts.SourceIsURL {
		options.WithURL(opts.Source)
	} else {
		f, err = os.Open(opts.Source)
		if err != nil {
			return nil, err
		}
	}
	return images.Import(ir.ClientCtx, f, options)
}

func (ir *ImageEngine) Push(ctx context.Context, source string, destination string, opts entities.ImagePushOptions) error {
	options := new(images.PushOptions)
	options.WithAll(opts.All).WithCompress(opts.Compress).WithUsername(opts.Username).WithPassword(opts.Password).WithAuthfile(opts.Authfile).WithFormat(opts.Format)

	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	return images.Push(ir.ClientCtx, source, destination, options)
}

func (ir *ImageEngine) Save(ctx context.Context, nameOrID string, tags []string, opts entities.ImageSaveOptions) error {
	var (
		f   *os.File
		err error
	)
	options := new(images.ExportOptions).WithFormat(opts.Format).WithCompress(opts.Compress)

	switch opts.Format {
	case "oci-dir", "docker-dir":
		f, err = ioutil.TempFile("", "podman_save")
		if err == nil {
			defer func() { _ = os.Remove(f.Name()) }()
		}
	default:
		// This code was added to allow for opening stdout replacing
		// os.Create(opts.Output) which was attempting to open the file
		// for read/write which fails on Darwin platforms
		f, err = os.OpenFile(opts.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	}
	if err != nil {
		return err
	}

	exErr := images.Export(ir.ClientCtx, append([]string{nameOrID}, tags...), f, options)
	if err := f.Close(); err != nil {
		return err
	}
	if exErr != nil {
		return exErr
	}

	if opts.Format != "oci-dir" && opts.Format != "docker-dir" {
		return nil
	}

	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}
	info, err := os.Stat(opts.Output)
	switch {
	case err == nil:
		if info.Mode().IsRegular() {
			return errors.Errorf("%q already exists as a regular file", opts.Output)
		}
	case os.IsNotExist(err):
		if err := os.Mkdir(opts.Output, 0755); err != nil {
			return err
		}
	default:
		return err
	}
	return utils2.UntarToFileSystem(opts.Output, f, nil)
}

func (ir *ImageEngine) Search(ctx context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	mappedFilters := make(map[string][]string)
	filters, err := libimage.ParseSearchFilter(opts.Filters)
	if err != nil {
		return nil, err
	}
	if stars := filters.Stars; stars > 0 {
		mappedFilters["stars"] = []string{strconv.Itoa(stars)}
	}

	if official := filters.IsOfficial; official != types.OptionalBoolUndefined {
		mappedFilters["is-official"] = []string{strconv.FormatBool(official == types.OptionalBoolTrue)}
	}

	if automated := filters.IsAutomated; automated != types.OptionalBoolUndefined {
		mappedFilters["is-automated"] = []string{strconv.FormatBool(automated == types.OptionalBoolTrue)}
	}

	options := new(images.SearchOptions)
	options.WithAuthfile(opts.Authfile).WithFilters(mappedFilters).WithLimit(opts.Limit)
	options.WithListTags(opts.ListTags).WithNoTrunc(opts.NoTrunc)
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	return images.Search(ir.ClientCtx, term, options)
}

func (ir *ImageEngine) Config(_ context.Context) (*config.Config, error) {
	return config.Default()
}

func (ir *ImageEngine) Build(_ context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	report, err := images.Build(ir.ClientCtx, containerFiles, opts)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (ir *ImageEngine) Tree(ctx context.Context, nameOrID string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	options := new(images.TreeOptions).WithWhatRequires(opts.WhatRequires)
	return images.Tree(ir.ClientCtx, nameOrID, options)
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
}

func (ir *ImageEngine) Sign(ctx context.Context, names []string, options entities.SignOptions) (*entities.SignReport, error) {
	return nil, errors.New("not implemented yet")
}
