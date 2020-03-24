// +build ABISupport

package abi

import (
	"context"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/filters"
	"github.com/containers/libpod/pkg/domain/infra/abi/parse"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) VolumeCreate(ctx context.Context, opts entities.VolumeCreateOptions) (*entities.IdOrNameResponse, error) {
	var (
		volumeOptions []libpod.VolumeCreateOption
	)
	if len(opts.Name) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(opts.Name))
	}
	if len(opts.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(opts.Driver))
	}
	if len(opts.Label) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(opts.Label))
	}
	if len(opts.Options) > 0 {
		parsedOptions, err := parse.ParseVolumeOptions(opts.Options)
		if err != nil {
			return nil, err
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}
	vol, err := ic.Libpod.NewVolume(ctx, volumeOptions...)
	if err != nil {
		return nil, err
	}
	return &entities.IdOrNameResponse{IdOrName: vol.Name()}, nil
}

func (ic *ContainerEngine) VolumeRm(ctx context.Context, namesOrIds []string, opts entities.VolumeRmOptions) ([]*entities.VolumeRmReport, error) {
	var (
		err     error
		reports []*entities.VolumeRmReport
		vols    []*libpod.Volume
	)
	if opts.All {
		vols, err = ic.Libpod.Volumes()
		if err != nil {
			return nil, err
		}
	} else {
		for _, id := range namesOrIds {
			vol, err := ic.Libpod.LookupVolume(id)
			if err != nil {
				reports = append(reports, &entities.VolumeRmReport{
					Err: err,
					Id:  id,
				})
				continue
			}
			vols = append(vols, vol)
		}
	}
	for _, vol := range vols {
		reports = append(reports, &entities.VolumeRmReport{
			Err: ic.Libpod.RemoveVolume(ctx, vol, opts.Force),
			Id:  vol.Name(),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumeInspect(ctx context.Context, namesOrIds []string, opts entities.VolumeInspectOptions) ([]*entities.VolumeInspectReport, error) {
	var (
		err     error
		reports []*entities.VolumeInspectReport
		vols    []*libpod.Volume
	)

	// Note: as with previous implementation, a single failure here
	// results a return.
	if opts.All {
		vols, err = ic.Libpod.GetAllVolumes()
		if err != nil {
			return nil, err
		}
	} else {
		for _, v := range namesOrIds {
			vol, err := ic.Libpod.LookupVolume(v)
			if err != nil {
				return nil, errors.Wrapf(err, "error inspecting volume %s", v)
			}
			vols = append(vols, vol)
		}
	}
	for _, v := range vols {
		config := entities.VolumeConfigResponse{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: v.MountPoint(),
			CreatedAt:  v.CreatedTime(),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
			UID:        v.UID(),
			GID:        v.GID(),
		}
		reports = append(reports, &entities.VolumeInspectReport{VolumeConfigResponse: &config})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumePrune(ctx context.Context, opts entities.VolumePruneOptions) ([]*entities.VolumePruneReport, error) {
	var (
		reports []*entities.VolumePruneReport
	)
	pruned, err := ic.Libpod.PruneVolumes(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range pruned {
		reports = append(reports, &entities.VolumePruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumeList(ctx context.Context, opts entities.VolumeListOptions) ([]*entities.VolumeListReport, error) {
	var (
		reports []*entities.VolumeListReport
	)
	volumeFilters, err := filters.GenerateVolumeFilters(opts.Filter)
	if err != nil {
		return nil, err
	}
	vols, err := ic.Libpod.Volumes(volumeFilters...)
	if err != nil {
		return nil, err
	}
	for _, v := range vols {
		config := entities.VolumeConfigResponse{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: v.MountPoint(),
			CreatedAt:  v.CreatedTime(),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
			UID:        v.UID(),
			GID:        v.GID(),
		}
		reports = append(reports, &entities.VolumeListReport{VolumeConfigResponse: config})
	}
	return reports, nil
}
