// +build ABISupport

package abi

import (
	"context"

	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/utils"
)

func (ir *ImageEngine) Delete(ctx context.Context, nameOrId string, opts entities.ImageDeleteOptions) (*entities.ImageDeleteReport, error) {
	image, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId)
	if err != nil {
		return nil, err
	}

	results, err := ir.Libpod.RemoveImage(ctx, image, opts.Force)
	if err != nil {
		return nil, err
	}

	report := entities.ImageDeleteReport{}
	if err := utils.DeepCopy(&report, results); err != nil {
		return nil, err
	}
	return &report, nil
}

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) (*entities.ImagePruneReport, error) {
	results, err := ir.Libpod.ImageRuntime().PruneImages(ctx, opts.All, []string{})
	if err != nil {
		return nil, err
	}

	report := entities.ImagePruneReport{}
	copy(report.Report.Id, results)
	return &report, nil
}

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) (*entities.ImageListReport, error) {
	var (
		images []*libpodImage.Image
		err    error
	)

	filters := utils.ToLibpodFilters(opts.Filters)
	if len(filters) > 0 {
		images, err = ir.Libpod.ImageRuntime().GetImagesWithFilters(filters)
	} else {
		images, err = ir.Libpod.ImageRuntime().GetImages()
	}
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
// 	if err := utils.DeepCopy(&report, results); err != nil {
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
