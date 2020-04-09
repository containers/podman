// +build varlink

package varlinkapi

import (
	"encoding/json"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	iopodman "github.com/containers/libpod/pkg/varlink"
)

// VolumeCreate creates a libpod volume based on input from a varlink connection
func (i *VarlinkAPI) VolumeCreate(call iopodman.VarlinkCall, options iopodman.VolumeCreateOpts) error {
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
		parsedOptions, err := shared.ParseVolumeOptions(options.Options)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}
	newVolume, err := i.Runtime.NewVolume(getContext(), volumeOptions...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyVolumeCreate(newVolume.Name())
}

// VolumeRemove removes volumes by options.All or options.Volumes
func (i *VarlinkAPI) VolumeRemove(call iopodman.VarlinkCall, options iopodman.VolumeRemoveOpts) error {
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
func (i *VarlinkAPI) GetVolumes(call iopodman.VarlinkCall, args []string, all bool) error {
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

// InspectVolume inspects a single volume, returning its JSON as a string.
func (i *VarlinkAPI) InspectVolume(call iopodman.VarlinkCall, name string) error {
	vol, err := i.Runtime.LookupVolume(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	inspectOut, err := vol.Inspect()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	inspectJSON, err := json.Marshal(inspectOut)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyInspectVolume(string(inspectJSON))
}

// VolumesPrune removes unused images via a varlink call
func (i *VarlinkAPI) VolumesPrune(call iopodman.VarlinkCall) error {
	var (
		prunedErrors []string
		prunedNames  []string
	)
	responses, err := i.Runtime.PruneVolumes(getContext())
	if err != nil {
		return call.ReplyVolumesPrune([]string{}, []string{err.Error()})
	}
	for k, v := range responses {
		if v == nil {
			prunedNames = append(prunedNames, k)
		} else {
			prunedErrors = append(prunedErrors, v.Error())
		}
	}
	return call.ReplyVolumesPrune(prunedNames, prunedErrors)
}
