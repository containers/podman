package shared

import (
	"context"
	"strconv"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Remove given set of volumes
func SharedRemoveVolumes(ctx context.Context, runtime *libpod.Runtime, vols []string, all, force bool) ([]string, map[string]error, error) {
	var (
		toRemove []*libpod.Volume
		success  []string
		failed   map[string]error
	)

	failed = make(map[string]error)

	if all {
		vols, err := runtime.Volumes()
		if err != nil {
			return nil, nil, err
		}
		toRemove = vols
	} else {
		for _, v := range vols {
			vol, err := runtime.LookupVolume(v)
			if err != nil {
				failed[v] = err
				continue
			}
			toRemove = append(toRemove, vol)
		}
	}

	// We could parallelize this, but I haven't heard anyone complain about
	// performance here yet, so hold off.
	for _, vol := range toRemove {
		if err := runtime.RemoveVolume(ctx, vol, force); err != nil {
			failed[vol.Name()] = err
			continue
		}
		success = append(success, vol.Name())
	}

	return success, failed, nil
}

// Handle volume options from CLI.
// Parse "o" option to find UID, GID.
func ParseVolumeOptions(opts map[string]string) ([]libpod.VolumeCreateOption, error) {
	libpodOptions := []libpod.VolumeCreateOption{}
	volumeOptions := make(map[string]string)

	for key, value := range opts {
		switch key {
		case "o":
			// o has special handling to parse out UID, GID.
			// These are separate Libpod options.
			splitVal := strings.Split(value, ",")
			finalVal := []string{}
			for _, o := range splitVal {
				// Options will be formatted as either "opt" or
				// "opt=value"
				splitO := strings.SplitN(o, "=", 2)
				switch strings.ToLower(splitO[0]) {
				case "uid":
					if len(splitO) != 2 {
						return nil, errors.Wrapf(define.ErrInvalidArg, "uid option must provide a UID")
					}
					intUID, err := strconv.Atoi(splitO[1])
					if err != nil {
						return nil, errors.Wrapf(err, "cannot convert UID %s to integer", splitO[1])
					}
					logrus.Debugf("Removing uid= from options and adding WithVolumeUID for UID %d", intUID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeUID(intUID))
				case "gid":
					if len(splitO) != 2 {
						return nil, errors.Wrapf(define.ErrInvalidArg, "gid option must provide a GID")
					}
					intGID, err := strconv.Atoi(splitO[1])
					if err != nil {
						return nil, errors.Wrapf(err, "cannot convert GID %s to integer", splitO[1])
					}
					logrus.Debugf("Removing gid= from options and adding WithVolumeGID for GID %d", intGID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeGID(intGID))
				default:
					finalVal = append(finalVal, o)
				}
			}
			if len(finalVal) > 0 {
				volumeOptions[key] = strings.Join(finalVal, ",")
			}
		default:
			volumeOptions[key] = value
		}
	}

	if len(volumeOptions) > 0 {
		libpodOptions = append(libpodOptions, libpod.WithVolumeOptions(volumeOptions))
	}

	return libpodOptions, nil
}
