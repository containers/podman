package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/volumes"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) VolumeCreate(ctx context.Context, opts entities.VolumeCreateOptions) (*entities.IdOrNameResponse, error) {
	response, err := volumes.Create(ic.ClientCxt, opts)
	if err != nil {
		return nil, err
	}
	return &entities.IdOrNameResponse{IdOrName: response.Name}, nil
}

func (ic *ContainerEngine) VolumeRm(ctx context.Context, namesOrIds []string, opts entities.VolumeRmOptions) ([]*entities.VolumeRmReport, error) {
	var (
		reports []*entities.VolumeRmReport
	)

	if opts.All {
		vols, err := volumes.List(ic.ClientCxt, nil)
		if err != nil {
			return nil, err
		}
		for _, v := range vols {
			namesOrIds = append(namesOrIds, v.Name)
		}
	}
	for _, id := range namesOrIds {
		reports = append(reports, &entities.VolumeRmReport{
			Err: volumes.Remove(ic.ClientCxt, id, &opts.Force),
			Id:  id,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumeInspect(ctx context.Context, namesOrIds []string, opts entities.VolumeInspectOptions) ([]*entities.VolumeInspectReport, error) {
	var (
		reports []*entities.VolumeInspectReport
	)
	if opts.All {
		vols, err := volumes.List(ic.ClientCxt, nil)
		if err != nil {
			return nil, err
		}
		for _, v := range vols {
			namesOrIds = append(namesOrIds, v.Name)
		}
	}
	for _, id := range namesOrIds {
		data, err := volumes.Inspect(ic.ClientCxt, id)
		if err != nil {
			return nil, err
		}
		reports = append(reports, &entities.VolumeInspectReport{VolumeConfigResponse: data})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumePrune(ctx context.Context, opts entities.VolumePruneOptions) ([]*entities.VolumePruneReport, error) {
	return volumes.Prune(ic.ClientCxt)
}

func (ic *ContainerEngine) VolumeList(ctx context.Context, opts entities.VolumeListOptions) ([]*entities.VolumeListReport, error) {
	return volumes.List(ic.ClientCxt, opts.Filter)
}
