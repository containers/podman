package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// VolumeCreate creates a libpod volume based on input from a varlink connection
func (i *LibpodAPI) VolumeCreate(call iopodman.VarlinkCall, options iopodman.VolumeCreateOpts) error {
	var volumeOptions []libpod.VolumeCreateOption

	if len(options.VolumeName) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(options.VolumeName))
	}
	if len(options.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(options.Driver))
	}
	if len(options.Labels) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(options.Labels))
	}
	if len(options.Options) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeOptions(options.Options))
	}
	newVolume, err := i.Runtime.NewVolume(getContext(), volumeOptions...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyVolumeCreate(newVolume.Name())
}
