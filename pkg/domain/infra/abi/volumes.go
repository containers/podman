package abi

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/entities/reports"
	"github.com/containers/podman/v5/pkg/domain/filters"
	"github.com/containers/podman/v5/pkg/domain/infra/abi/parse"
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

	if opts.IgnoreIfExists {
		volumeOptions = append(volumeOptions, libpod.WithVolumeIgnoreIfExist())
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
				if opts.Ignore && errors.Is(err, define.ErrNoSuchVolume) {
					continue
				}
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
			Err: ic.Libpod.RemoveVolume(ctx, vol, opts.Force, opts.Timeout),
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
				if errors.Is(err, define.ErrNoSuchVolume) {
					errs = append(errs, fmt.Errorf("no such volume %s", v))
					continue
				} else {
					return nil, nil, fmt.Errorf("inspecting volume %s: %w", v, err)
				}
			}
			vols = append(vols, vol)
		}
	}
	reports := make([]*entities.VolumeInspectReport, 0, len(vols))
	for _, v := range vols {
		inspectOut, err := v.Inspect()
		if err != nil {
			return nil, nil, err
		}
		config := entities.VolumeConfigResponse{
			InspectVolumeData: *inspectOut,
		}
		reports = append(reports, &entities.VolumeInspectReport{VolumeConfigResponse: &config})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) VolumePrune(ctx context.Context, options entities.VolumePruneOptions) ([]*reports.PruneReport, error) {
	funcs := []libpod.VolumeFilter{}
	for filter, filterValues := range options.Filters {
		filterFunc, err := filters.GenerateVolumeFilters(filter, filterValues, ic.Libpod)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, filterFunc)
	}
	return ic.pruneVolumesHelper(ctx, funcs)
}

func (ic *ContainerEngine) pruneVolumesHelper(ctx context.Context, filterFuncs []libpod.VolumeFilter) ([]*reports.PruneReport, error) {
	pruned, err := ic.Libpod.PruneVolumes(ctx, filterFuncs)
	if err != nil {
		return nil, err
	}
	return pruned, nil
}

func (ic *ContainerEngine) VolumeList(ctx context.Context, opts entities.VolumeListOptions) ([]*entities.VolumeListReport, error) {
	volumeFilters := []libpod.VolumeFilter{}
	for filter, value := range opts.Filter {
		filterFunc, err := filters.GenerateVolumeFilters(filter, value, ic.Libpod)
		if err != nil {
			return nil, err
		}
		volumeFilters = append(volumeFilters, filterFunc)
	}

	vols, err := ic.Libpod.Volumes(volumeFilters...)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.VolumeListReport, 0, len(vols))
	for _, v := range vols {
		inspectOut, err := v.Inspect()
		if err != nil {
			return nil, err
		}
		config := entities.VolumeConfigResponse{
			InspectVolumeData: *inspectOut,
		}
		reports = append(reports, &entities.VolumeListReport{VolumeConfigResponse: config})
	}
	return reports, nil
}

// VolumeExists check if a given volume name exists
func (ic *ContainerEngine) VolumeExists(ctx context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := ic.Libpod.HasVolume(nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}

// Volumemounted check if a given volume using plugin or filesystem is mounted or not.
func (ic *ContainerEngine) VolumeMounted(ctx context.Context, nameOrID string) (*entities.BoolReport, error) {
	vol, err := ic.Libpod.LookupVolume(nameOrID)
	if err != nil {
		return nil, err
	}
	mountCount, err := vol.MountCount()
	if err != nil {
		// FIXME: this error should probably be returned
		return &entities.BoolReport{Value: false}, nil //nolint: nilerr
	}
	if mountCount > 0 {
		return &entities.BoolReport{Value: true}, nil
	}
	return &entities.BoolReport{Value: false}, nil
}

func (ic *ContainerEngine) VolumeMount(ctx context.Context, nameOrIDs []string) ([]*entities.VolumeMountReport, error) {
	reports := []*entities.VolumeMountReport{}
	for _, name := range nameOrIDs {
		report := entities.VolumeMountReport{Id: name}
		vol, err := ic.Libpod.LookupVolume(name)
		if err != nil {
			report.Err = err
		} else {
			report.Path, report.Err = vol.Mount()
		}
		reports = append(reports, &report)
	}

	return reports, nil
}

func (ic *ContainerEngine) VolumeUnmount(ctx context.Context, nameOrIDs []string) ([]*entities.VolumeUnmountReport, error) {
	reports := []*entities.VolumeUnmountReport{}
	for _, name := range nameOrIDs {
		report := entities.VolumeUnmountReport{Id: name}
		vol, err := ic.Libpod.LookupVolume(name)
		if err != nil {
			report.Err = err
		} else {
			report.Err = vol.Unmount()
		}
		reports = append(reports, &report)
	}

	return reports, nil
}

func (ic *ContainerEngine) VolumeReload(ctx context.Context) (*entities.VolumeReloadReport, error) {
	report := ic.Libpod.UpdateVolumePlugins(ctx)
	return &entities.VolumeReloadReport{VolumeReload: *report}, nil
}
