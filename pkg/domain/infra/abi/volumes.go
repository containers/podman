package abi

import (
	"context"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/filters"
	"github.com/containers/podman/v2/pkg/domain/infra/abi/parse"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) VolumeCreate(ctx context.Context, opts entities.VolumeCreateOptions) (*entities.IDOrNameResponse, error) {
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
		parsedOptions, err := parse.VolumeOptions(opts.Options)
		if err != nil {
			return nil, err
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}
	vol, err := ic.Libpod.NewVolume(ctx, volumeOptions...)
	if err != nil {
		return nil, err
	}
	return &entities.IDOrNameResponse{IDOrName: vol.Name()}, nil
}

func (ic *ContainerEngine) VolumeRm(ctx context.Context, namesOrIds []string, opts entities.VolumeRmOptions) ([]*entities.VolumeRmReport, error) {
	var (
		err     error
		vols    []*libpod.Volume
		reports = []*entities.VolumeRmReport{}
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

func (ic *ContainerEngine) VolumeInspect(ctx context.Context, namesOrIds []string, opts entities.InspectOptions) ([]*entities.VolumeInspectReport, []error, error) {
	var (
		err  error
		errs []error
		vols []*libpod.Volume
	)

	// Note: as with previous implementation, a single failure here
	// results a return.
	if opts.All {
		vols, err = ic.Libpod.GetAllVolumes()
		if err != nil {
			return nil, nil, err
		}
	} else {
		for _, v := range namesOrIds {
			vol, err := ic.Libpod.LookupVolume(v)
			if err != nil {
				if errors.Cause(err) == define.ErrNoSuchVolume {
					errs = append(errs, errors.Errorf("no such volume %s", v))
					continue
				} else {
					return nil, nil, errors.Wrapf(err, "error inspecting volume %s", v)
				}
			}
			vols = append(vols, vol)
		}
	}
	reports := make([]*entities.VolumeInspectReport, 0, len(vols))
	for _, v := range vols {
		var uid, gid int
		uid, err = v.UID()
		if err != nil {
			return nil, nil, err
		}
		gid, err = v.GID()
		if err != nil {
			return nil, nil, err
		}
		config := entities.VolumeConfigResponse{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: v.MountPoint(),
			CreatedAt:  v.CreatedTime(),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
			UID:        uid,
			GID:        gid,
		}
		reports = append(reports, &entities.VolumeInspectReport{VolumeConfigResponse: &config})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) VolumePrune(ctx context.Context) ([]*entities.VolumePruneReport, error) {
	return ic.pruneVolumesHelper(ctx)
}

func (ic *ContainerEngine) pruneVolumesHelper(ctx context.Context) ([]*entities.VolumePruneReport, error) {
	pruned, err := ic.Libpod.PruneVolumes(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.VolumePruneReport, 0, len(pruned))
	for k, v := range pruned {
		reports = append(reports, &entities.VolumePruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumeList(ctx context.Context, opts entities.VolumeListOptions) ([]*entities.VolumeListReport, error) {
	volumeFilters, err := filters.GenerateVolumeFilters(opts.Filter)
	if err != nil {
		return nil, err
	}
	vols, err := ic.Libpod.Volumes(volumeFilters...)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.VolumeListReport, 0, len(vols))
	for _, v := range vols {
		var uid, gid int
		uid, err = v.UID()
		if err != nil {
			return nil, err
		}
		gid, err = v.GID()
		if err != nil {
			return nil, err
		}
		config := entities.VolumeConfigResponse{
			Name:       v.Name(),
			Driver:     v.Driver(),
			Mountpoint: v.MountPoint(),
			CreatedAt:  v.CreatedTime(),
			Labels:     v.Labels(),
			Scope:      v.Scope(),
			Options:    v.Options(),
			UID:        uid,
			GID:        gid,
		}
		reports = append(reports, &entities.VolumeListReport{VolumeConfigResponse: config})
	}
	return reports, nil
}
