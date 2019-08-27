// +build varlink

package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/shared"
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

// VolumeRemove removes volumes by options.All or options.Volumes
func (i *LibpodAPI) VolumeRemove(call iopodman.VarlinkCall, options iopodman.VolumeRemoveOpts) error {
	success, failed, err := shared.SharedRemoveVolumes(getContext(), i.Runtime, options.Volumes, options.All, options.Force)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	// Convert map[string]string to map[string]error
	errStrings := make(map[string]string)
	for k, v := range failed {
		errStrings[k] = v.Error()
	}
	return call.ReplyVolumeRemove(success, errStrings)
}

// GetVolumes returns all the volumes known to the remote system
func (i *LibpodAPI) GetVolumes(call iopodman.VarlinkCall, args []string, all bool) error {
	var (
		err     error
		reply   []*libpod.Volume
		volumes []iopodman.Volume
	)
	if all {
		reply, err = i.Runtime.GetAllVolumes()
	} else {
		for _, v := range args {
			vol, err := i.Runtime.GetVolume(v)
			if err != nil {
				return err
			}
			reply = append(reply, vol)
		}
	}
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	// Build the iopodman.volume struct for the return
	for _, v := range reply {
		newVol := iopodman.Volume{
			Driver:     v.Driver(),
			Labels:     v.Labels(),
			MountPoint: v.MountPoint(),
			Name:       v.Name(),
			Options:    v.Options(),
		}
		volumes = append(volumes, newVol)
	}
	return call.ReplyGetVolumes(volumes)
}

// VolumesPrune removes unused images via a varlink call
func (i *LibpodAPI) VolumesPrune(call iopodman.VarlinkCall) error {
	var errs []string
	prunedNames, prunedErrors := i.Runtime.PruneVolumes(getContext())
	if len(prunedErrors) == 0 {
		return call.ReplyVolumesPrune(prunedNames, []string{})
	}

	// We need to take the errors and capture their strings to go back over
	// varlink
	for _, e := range prunedErrors {
		errs = append(errs, e.Error())
	}
	return call.ReplyVolumesPrune(prunedNames, errs)
}
