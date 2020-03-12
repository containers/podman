package tunnel

import (
	"context"
	"net/url"

	images "github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/utils"
)

func (ir *ImageEngine) Delete(ctx context.Context, nameOrId string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
	results, err := images.Remove(ir.ClientCxt, nameOrId, &opts.Force)
	if err != nil {
		return nil, err
	}

	report := entities.ImageDeleteReport{
		Untagged: nil,
		Deleted:  "",
	}

	for _, e := range results {
		if a, ok := e["Deleted"]; ok {
			report.Deleted = a
		}

		if a, ok := e["Untagged"]; ok {
			report.Untagged = append(report.Untagged, a)
		}
	}
	return &report, err
}

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) (*entities.ImageListReport, error) {
	images, err := images.List(ir.ClientCxt, &opts.All, opts.Filters)
	if err != nil {
		return nil, err
	}

	report := entities.ImageListReport{
		Images: make([]entities.ImageSummary, len(images)),
	}
	for i, img := range images {
		hold := entities.ImageSummary{}
		if err := utils.DeepCopy(&hold, img); err != nil {
			return nil, err
		}
		report.Images[i] = hold
	}
	return &report, nil
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
	results, err := images.Prune(ir.ClientCxt, url.Values{})
	if err != nil {
		return nil, err
	}

	report := entities.ImagePruneReport{}
	copy(report.Report.Id, results)
	return &report, nil
}
