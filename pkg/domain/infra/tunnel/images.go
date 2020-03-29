package tunnel

import (
	"context"

	"github.com/containers/image/v5/docker/reference"
	images "github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/utils"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrId string) (*entities.BoolReport, error) {
	found, err := images.Exists(ir.ClientCxt, nameOrId)
	return &entities.BoolReport{Value: found}, err
}

func (ir *ImageEngine) Delete(ctx context.Context, nameOrId []string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
	report := entities.ImageDeleteReport{}

	for _, id := range nameOrId {
		results, err := images.Remove(ir.ClientCxt, id, &opts.Force)
		if err != nil {
			return nil, err
		}
		for _, e := range results {
			if a, ok := e["Deleted"]; ok {
				report.Deleted = append(report.Deleted, a)
			}

			if a, ok := e["Untagged"]; ok {
				report.Untagged = append(report.Untagged, a)
			}
		}
	}
	return &report, nil
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
