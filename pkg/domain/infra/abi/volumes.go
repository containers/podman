// +build ABISupport

package abi

import (
	"context"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi/parse"
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
