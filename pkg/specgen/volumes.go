package specgen

import (
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/parse"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NamedVolume holds information about a named volume that will be mounted into
// the container.
type NamedVolume struct {
	// Name is the name of the named volume to be mounted. May be empty.
	// If empty, a new named volume with a pseudorandomly generated name
	// will be mounted at the given destination.
	Name string
	// Destination to mount the named volume within the container. Must be
	// an absolute path. Path will be created if it does not exist.
	Dest string
	// Options are options that the named volume will be mounted with.
	Options []string
}

// OverlayVolume holds information about a overlay volume that will be mounted into
// the container.
type OverlayVolume struct {
	// Destination is the absolute path where the mount will be placed in the container.
	Destination string `json:"destination"`
	// Source specifies the source path of the mount.
	Source string `json:"source,omitempty"`
	// Options holds overlay volume options.
	Options []string `json:"options,omitempty"`
}

// ImageVolume is a volume based on a container image.  The container image is
// first mounted on the host and is then bind-mounted into the container.  An
// ImageVolume is always mounted read only.
type ImageVolume struct {
	// Source is the source of the image volume.  The image can be referred
	// to by name and by ID.
	Source string
	// Destination is the absolute path of the mount in the container.
	Destination string
	// ReadWrite sets the volume writable.
	ReadWrite bool
}

// GenVolumeMounts parses user input into mounts, volumes and overlay volumes
func GenVolumeMounts(volumeFlag []string) (map[string]spec.Mount, map[string]*NamedVolume, map[string]*OverlayVolume, error) {
	errDuplicateDest := errors.Errorf("duplicate mount destination")

	mounts := make(map[string]spec.Mount)
	volumes := make(map[string]*NamedVolume)
	overlayVolumes := make(map[string]*OverlayVolume)

	volumeFormatErr := errors.Errorf("incorrect volume format, should be [host-dir:]ctr-dir[:option]")

	for _, vol := range volumeFlag {
		var (
			options []string
			src     string
			dest    string
			err     error
		)

		splitVol := strings.Split(vol, ":")
		if len(splitVol) > 3 {
			return nil, nil, nil, errors.Wrapf(volumeFormatErr, vol)
		}

		src = splitVol[0]
		if len(splitVol) == 1 {
			// This is an anonymous named volume. Only thing given
			// is destination.
			// Name/source will be blank, and populated by libpod.
			src = ""
			dest = splitVol[0]
		} else if len(splitVol) > 1 {
			dest = splitVol[1]
		}
		if len(splitVol) > 2 {
			if options, err = parse.ValidateVolumeOpts(strings.Split(splitVol[2], ",")); err != nil {
				return nil, nil, nil, err
			}
		}

		// Do not check source dir for anonymous volumes
		if len(splitVol) > 1 {
			if len(src) == 0 {
				return nil, nil, nil, errors.New("host directory cannot be empty")
			}
		}
		if err := parse.ValidateVolumeCtrDir(dest); err != nil {
			return nil, nil, nil, err
		}

		cleanDest := filepath.Clean(dest)

		if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
			// This is not a named volume
			overlayFlag := false
			chownFlag := false
			for _, o := range options {
				if o == "O" {
					overlayFlag = true

					joinedOpts := strings.Join(options, "")
					if strings.Contains(joinedOpts, "U") {
						chownFlag = true
					}

					if len(options) > 2 || (len(options) == 2 && !chownFlag) {
						return nil, nil, nil, errors.New("can't use 'O' with other options")
					}
				}
			}
			if overlayFlag {
				// This is a overlay volume
				newOverlayVol := new(OverlayVolume)
				newOverlayVol.Destination = cleanDest
				newOverlayVol.Source = src
				newOverlayVol.Options = options

				if _, ok := overlayVolumes[newOverlayVol.Destination]; ok {
					return nil, nil, nil, errors.Wrapf(errDuplicateDest, newOverlayVol.Destination)
				}
				overlayVolumes[newOverlayVol.Destination] = newOverlayVol
			} else {
				newMount := spec.Mount{
					Destination: cleanDest,
					Type:        "bind",
					Source:      src,
					Options:     options,
				}
				if _, ok := mounts[newMount.Destination]; ok {
					return nil, nil, nil, errors.Wrapf(errDuplicateDest, newMount.Destination)
				}
				mounts[newMount.Destination] = newMount
			}
		} else {
			// This is a named volume
			newNamedVol := new(NamedVolume)
			newNamedVol.Name = src
			newNamedVol.Dest = cleanDest
			newNamedVol.Options = options

			if _, ok := volumes[newNamedVol.Dest]; ok {
				return nil, nil, nil, errors.Wrapf(errDuplicateDest, newNamedVol.Dest)
			}
			volumes[newNamedVol.Dest] = newNamedVol
		}

		logrus.Debugf("User mount %s:%s options %v", src, dest, options)
	}

	return mounts, volumes, overlayVolumes, nil
}
