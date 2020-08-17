package parse

import (
	"strconv"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Handle volume options from CLI.
// Parse "o" option to find UID, GID.
func VolumeOptions(opts map[string]string) ([]libpod.VolumeCreateOption, error) {
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
