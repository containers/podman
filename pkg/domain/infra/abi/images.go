// +build ABISupport

package abi

import (
	"context"
	"fmt"

	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrId string) (*entities.BoolReport, error) {
	if _, err := ir.Libpod.ImageRuntime().NewFromLocal(nameOrId); err != nil {
		return &entities.BoolReport{}, nil
	}
	return &entities.BoolReport{Value: true}, nil
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
	results, err := ir.Libpod.ImageRuntime().PruneImages(ctx, opts.All, []string{})
	if err != nil {
		return nil, err
	}

	report := entities.ImagePruneReport{}
	copy(report.Report.Id, results)
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
