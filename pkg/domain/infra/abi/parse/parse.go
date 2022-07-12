package parse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	units "github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

// Handle volume options from CLI.
// Parse "o" option to find UID, GID, Size.
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
				case "size":
					size, err := units.FromHumanSize(splitO[1])
					if err != nil {
						return nil, fmt.Errorf("cannot convert size %s to integer: %w", splitO[1], err)
					}
					libpodOptions = append(libpodOptions, libpod.WithVolumeSize(uint64(size)))
					finalVal = append(finalVal, o)
					// set option "SIZE": "$size"
					volumeOptions["SIZE"] = splitO[1]
				case "inodes":
					inodes, err := strconv.ParseUint(splitO[1], 10, 64)
					if err != nil {
						return nil, fmt.Errorf("cannot convert inodes %s to integer: %w", splitO[1], err)
					}
					libpodOptions = append(libpodOptions, libpod.WithVolumeInodes(inodes))
					finalVal = append(finalVal, o)
					// set option "INODES": "$size"
					volumeOptions["INODES"] = splitO[1]
				case "uid":
					if len(splitO) != 2 {
						return nil, fmt.Errorf("uid option must provide a UID: %w", define.ErrInvalidArg)
					}
					intUID, err := strconv.Atoi(splitO[1])
					if err != nil {
						return nil, fmt.Errorf("cannot convert UID %s to integer: %w", splitO[1], err)
					}
					logrus.Debugf("Removing uid= from options and adding WithVolumeUID for UID %d", intUID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeUID(intUID), libpod.WithVolumeNoChown())
					finalVal = append(finalVal, o)
					// set option "UID": "$uid"
					volumeOptions["UID"] = splitO[1]
				case "gid":
					if len(splitO) != 2 {
						return nil, fmt.Errorf("gid option must provide a GID: %w", define.ErrInvalidArg)
					}
					intGID, err := strconv.Atoi(splitO[1])
					if err != nil {
						return nil, fmt.Errorf("cannot convert GID %s to integer: %w", splitO[1], err)
					}
					logrus.Debugf("Removing gid= from options and adding WithVolumeGID for GID %d", intGID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeGID(intGID), libpod.WithVolumeNoChown())
					finalVal = append(finalVal, o)
					// set option "GID": "$gid"
					volumeOptions["GID"] = splitO[1]
				case "noquota":
					logrus.Debugf("Removing noquota from options and adding WithVolumeDisableQuota")
					libpodOptions = append(libpodOptions, libpod.WithVolumeDisableQuota())
					// set option "NOQUOTA": "true"
					volumeOptions["NOQUOTA"] = "true"
				case "timeout":
					if len(splitO) != 2 {
						return nil, fmt.Errorf("timeout option must provide a valid timeout in seconds: %w", define.ErrInvalidArg)
					}
					intTimeout, err := strconv.Atoi(splitO[1])
					if err != nil {
						return nil, fmt.Errorf("cannot convert Timeout %s to an integer: %w", splitO[1], err)
					}
					logrus.Debugf("Removing timeout from options and adding WithTimeout for Timeout %d", intTimeout)
					libpodOptions = append(libpodOptions, libpod.WithVolumeDriverTimeout(intTimeout))
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
