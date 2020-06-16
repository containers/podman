package tunnel

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/libpod/pkg/bindings"
	images "github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/utils"
	utils2 "github.com/containers/libpod/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func (ir *ImageEngine) History(ctx context.Context, nameOrID string, opts entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	results, err := images.History(ir.ClientCxt, nameOrID)
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
			return err
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
	// Remove all tags if none are provided
	if len(tags) == 0 {
		newImage, err := images.GetImage(ir.ClientCxt, nameOrID, bindings.PFalse)
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
			return errors.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Untag(ir.ClientCxt, nameOrID, tag, repo); err != nil {
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

	exErr := images.Export(ir.ClientCxt, nameOrID, f, &options.Format, &options.Compress)
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

func (ir *ImageEngine) Build(ctx context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	var tarReader io.Reader
	tarfile, err := archive.Tar(opts.ContextDirectory, 0)
	if err != nil {
		return nil, err
	}
	tarReader = tarfile
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if cwd != opts.ContextDirectory {
		fn := func(h *tar.Header, r io.Reader) (data []byte, update bool, skip bool, err error) {
			h.Name = filepath.Join(filepath.Base(opts.ContextDirectory), h.Name)
			return nil, false, false, nil
		}
		tarReader, err = transformArchive(tarfile, false, fn)
		if err != nil {
			return nil, err
		}
	}
	return images.Build(ir.ClientCxt, containerFiles, opts, tarReader)
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

// Sourced from openshift image builder

// TransformFileFunc is given a chance to transform an arbitrary input file.
type TransformFileFunc func(h *tar.Header, r io.Reader) (data []byte, update bool, skip bool, err error)

// filterArchive transforms the provided input archive to a new archive,
// giving the fn a chance to transform arbitrary files.
func filterArchive(r io.Reader, w io.Writer, fn TransformFileFunc) error {
	tr := tar.NewReader(r)
	tw := tar.NewWriter(w)

	var body io.Reader = tr

	for {
		h, err := tr.Next()
		if err == io.EOF {
			return tw.Close()
		}
		if err != nil {
			return err
		}

		name := h.Name
		data, ok, skip, err := fn(h, tr)
		logrus.Debugf("Transform %q -> %q: data=%t ok=%t skip=%t err=%v", name, h.Name, data != nil, ok, skip, err)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if ok {
			h.Size = int64(len(data))
			body = bytes.NewBuffer(data)
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := io.Copy(tw, body); err != nil {
			return err
		}
	}
}

func transformArchive(r io.Reader, compressed bool, fn TransformFileFunc) (io.Reader, error) {
	var cwe error
	pr, pw := io.Pipe()
	go func() {
		if compressed {
			in, err := archive.DecompressStream(r)
			if err != nil {
				cwe = pw.CloseWithError(err)
				return
			}
			r = in
		}
		err := filterArchive(r, pw, fn)
		cwe = pw.CloseWithError(err)
	}()
	return pr, cwe
}
