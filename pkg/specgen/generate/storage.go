package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var errDuplicateDest = errors.New("duplicate mount destination")

// Produce final mounts and named volumes for a container
func finalizeMounts(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, img *libimage.Image) ([]spec.Mount, []*specgen.NamedVolume, []*specgen.OverlayVolume, error) {
	// Get image volumes
	baseMounts, baseVolumes, err := getImageVolumes(ctx, img, s)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get volumes-from mounts
	volFromMounts, volFromVolumes, err := getVolumesFrom(s.VolumesFrom, rt)
	if err != nil {
		return nil, nil, nil, err
	}

	// Supersede from --volumes-from.
	for dest, mount := range volFromMounts {
		baseMounts[dest] = mount
	}
	for dest, volume := range volFromVolumes {
		baseVolumes[dest] = volume
	}

	// Need to make map forms of specgen mounts/volumes.
	unifiedMounts := map[string]spec.Mount{}
	unifiedVolumes := map[string]*specgen.NamedVolume{}
	unifiedOverlays := map[string]*specgen.OverlayVolume{}

	// Need to make map forms of specgen mounts/volumes.
	commonMounts, commonVolumes, commonOverlayVolumes, err := specgen.GenVolumeMounts(rtc.Volumes())
	if err != nil {
		return nil, nil, nil, err
	}

	for _, m := range s.Mounts {
		// Ensure that mount dest is clean, so that it can be
		// compared against named volumes and avoid duplicate mounts.
		if err = parse.ValidateVolumeCtrDir(m.Destination); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(m.Destination)
		if _, ok := unifiedMounts[cleanDestination]; ok {
			return nil, nil, nil, fmt.Errorf("conflict in specified mounts - multiple mounts at %q: %w", cleanDestination, errDuplicateDest)
		}
		unifiedMounts[cleanDestination] = m
	}

	for _, m := range commonMounts {
		if err = parse.ValidateVolumeCtrDir(m.Destination); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(m.Destination)
		if _, ok := unifiedMounts[cleanDestination]; !ok {
			unifiedMounts[cleanDestination] = m
		}
	}

	for _, v := range s.Volumes {
		if err = parse.ValidateVolumeCtrDir(v.Dest); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(v.Dest)
		if _, ok := unifiedVolumes[cleanDestination]; ok {
			return nil, nil, nil, fmt.Errorf("conflict in specified volumes - multiple volumes at %q: %w", cleanDestination, errDuplicateDest)
		}
		unifiedVolumes[cleanDestination] = v
	}

	for _, v := range commonVolumes {
		if err = parse.ValidateVolumeCtrDir(v.Dest); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(v.Dest)
		if _, ok := unifiedVolumes[cleanDestination]; !ok {
			unifiedVolumes[cleanDestination] = v
		}
	}

	for _, v := range s.OverlayVolumes {
		if err = parse.ValidateVolumeCtrDir(v.Destination); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(v.Destination)
		if _, ok := unifiedOverlays[cleanDestination]; ok {
			return nil, nil, nil, fmt.Errorf("conflict in specified volumes - multiple volumes at %q: %w", cleanDestination, errDuplicateDest)
		}
		unifiedOverlays[cleanDestination] = v
	}

	for _, v := range commonOverlayVolumes {
		if err = parse.ValidateVolumeCtrDir(v.Destination); err != nil {
			return nil, nil, nil, err
		}
		cleanDestination := filepath.Clean(v.Destination)
		if _, ok := unifiedOverlays[cleanDestination]; !ok {
			unifiedOverlays[cleanDestination] = v
		}
	}

	// If requested, add container init binary
	if s.Init {
		initPath := s.InitPath
		if initPath == "" && rtc != nil {
			initPath = rtc.Engine.InitPath
		}
		initMount, err := addContainerInitBinary(s, initPath)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, ok := unifiedMounts[initMount.Destination]; ok {
			return nil, nil, nil, fmt.Errorf("conflict with mount added by --init to %q: %w", initMount.Destination, errDuplicateDest)
		}
		unifiedMounts[initMount.Destination] = initMount
	}

	// Before superseding, we need to find volume mounts which conflict with
	// named volumes, and vice versa.
	// We'll delete the conflicts here as we supersede.
	for dest := range unifiedMounts {
		delete(baseVolumes, dest)
	}
	for dest := range unifiedVolumes {
		delete(baseMounts, dest)
	}

	// Supersede volumes-from/image volumes with unified volumes from above.
	// This is an unconditional replacement.
	for dest, mount := range unifiedMounts {
		baseMounts[dest] = mount
	}
	for dest, volume := range unifiedVolumes {
		baseVolumes[dest] = volume
	}

	// TODO: Investigate moving readonlyTmpfs into here. Would be more
	// correct.

	// Check for conflicts between named volumes and mounts
	for dest := range baseMounts {
		if _, ok := baseVolumes[dest]; ok {
			return nil, nil, nil, fmt.Errorf("conflict at mount destination %v: %w", dest, errDuplicateDest)
		}
	}
	for dest := range baseVolumes {
		if _, ok := baseMounts[dest]; ok {
			return nil, nil, nil, fmt.Errorf("conflict at mount destination %v: %w", dest, errDuplicateDest)
		}
	}
	// Final step: maps to arrays
	finalMounts := make([]spec.Mount, 0, len(baseMounts))
	for _, mount := range baseMounts {
		if mount.Type == define.TypeBind {
			absSrc, err := filepath.Abs(mount.Source)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("error getting absolute path of %s: %w", mount.Source, err)
			}
			mount.Source = absSrc
		}
		finalMounts = append(finalMounts, mount)
	}
	finalVolumes := make([]*specgen.NamedVolume, 0, len(baseVolumes))
	for _, volume := range baseVolumes {
		finalVolumes = append(finalVolumes, volume)
	}

	finalOverlays := make([]*specgen.OverlayVolume, 0, len(unifiedOverlays))
	for _, volume := range unifiedOverlays {
		finalOverlays = append(finalOverlays, volume)
	}

	return finalMounts, finalVolumes, finalOverlays, nil
}

// Get image volumes from the given image
func getImageVolumes(ctx context.Context, img *libimage.Image, s *specgen.SpecGenerator) (map[string]spec.Mount, map[string]*specgen.NamedVolume, error) {
	mounts := make(map[string]spec.Mount)
	volumes := make(map[string]*specgen.NamedVolume)

	mode := strings.ToLower(s.ImageVolumeMode)

	// Image may be nil (rootfs in use), or image volume mode may be ignore.
	if img == nil || mode == "ignore" {
		return mounts, volumes, nil
	}

	inspect, err := img.Inspect(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error inspecting image to get image volumes: %w", err)
	}
	for volume := range inspect.Config.Volumes {
		logrus.Debugf("Image has volume at %q", volume)
		cleanDest := filepath.Clean(volume)
		switch mode {
		case "", "anonymous":
			// Anonymous volumes have no name.
			newVol := new(specgen.NamedVolume)
			newVol.Dest = cleanDest
			newVol.Options = []string{"rprivate", "rw", "nodev", "exec"}
			volumes[cleanDest] = newVol
			logrus.Debugf("Adding anonymous image volume at %q", cleanDest)
		case "tmpfs":
			mount := spec.Mount{
				Destination: cleanDest,
				Source:      define.TypeTmpfs,
				Type:        define.TypeTmpfs,
				Options:     []string{"rprivate", "rw", "nodev", "exec"},
			}
			mounts[cleanDest] = mount
			logrus.Debugf("Adding tmpfs image volume at %q", cleanDest)
		}
	}

	return mounts, volumes, nil
}

func getVolumesFrom(volumesFrom []string, runtime *libpod.Runtime) (map[string]spec.Mount, map[string]*specgen.NamedVolume, error) {
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*specgen.NamedVolume)

	for _, volume := range volumesFrom {
		var options []string

		splitVol := strings.SplitN(volume, ":", 2)
		if len(splitVol) == 2 {
			splitOpts := strings.Split(splitVol[1], ",")
			setRORW := false
			setZ := false
			for _, opt := range splitOpts {
				switch opt {
				case "z":
					if setZ {
						return nil, nil, errors.New("cannot set :z more than once in mount options")
					}
					setZ = true
				case "ro", "rw":
					if setRORW {
						return nil, nil, errors.New("cannot set ro or rw options more than once")
					}
					setRORW = true
				default:
					return nil, nil, fmt.Errorf("invalid option %q specified - volumes from another container can only use z,ro,rw options", opt)
				}
			}
			options = splitOpts
		}

		ctr, err := runtime.LookupContainer(splitVol[0])
		if err != nil {
			return nil, nil, fmt.Errorf("error looking up container %q for volumes-from: %w", splitVol[0], err)
		}

		logrus.Debugf("Adding volumes from container %s", ctr.ID())

		// Look up the container's user volumes. This gets us the
		// destinations of all mounts the user added to the container.
		userVolumesArr := ctr.UserVolumes()

		// We're going to need to access them a lot, so convert to a map
		// to reduce looping.
		// We'll also use the map to indicate if we missed any volumes along the way.
		userVolumes := make(map[string]bool)
		for _, dest := range userVolumesArr {
			userVolumes[dest] = false
		}

		// Now we get the container's spec and loop through its volumes
		// and append them in if we can find them.
		spec := ctr.Spec()
		if spec == nil {
			return nil, nil, fmt.Errorf("retrieving container %s spec for volumes-from", ctr.ID())
		}
		for _, mnt := range spec.Mounts {
			if mnt.Type != define.TypeBind {
				continue
			}
			if _, exists := userVolumes[mnt.Destination]; exists {
				userVolumes[mnt.Destination] = true

				if len(options) != 0 {
					mnt.Options = options
				}

				if _, ok := finalMounts[mnt.Destination]; ok {
					logrus.Debugf("Overriding mount to %s with new mount from container %s", mnt.Destination, ctr.ID())
				}
				finalMounts[mnt.Destination] = mnt
			}
		}

		// We're done with the spec mounts. Add named volumes.
		// Add these unconditionally - none of them are automatically
		// part of the container, as some spec mounts are.
		namedVolumes := ctr.NamedVolumes()
		for _, namedVol := range namedVolumes {
			if _, exists := userVolumes[namedVol.Dest]; exists {
				userVolumes[namedVol.Dest] = true
			}

			if len(options) != 0 {
				namedVol.Options = options
			}

			if _, ok := finalMounts[namedVol.Dest]; ok {
				logrus.Debugf("Overriding named volume mount to %s with new named volume from container %s", namedVol.Dest, ctr.ID())
			}
			if err = parse.ValidateVolumeCtrDir(namedVol.Dest); err != nil {
				return nil, nil, err
			}

			cleanDest := filepath.Clean(namedVol.Dest)
			newVol := new(specgen.NamedVolume)
			newVol.Dest = cleanDest
			newVol.Options = namedVol.Options
			newVol.Name = namedVol.Name

			finalNamedVolumes[namedVol.Dest] = newVol
		}

		// Check if we missed any volumes
		for volDest, found := range userVolumes {
			if !found {
				logrus.Warnf("Unable to match volume %s from container %s for volumes-from", volDest, ctr.ID())
			}
		}
	}

	return finalMounts, finalNamedVolumes, nil
}

// AddContainerInitBinary adds the init binary specified by path iff the
// container will run in a private PID namespace that is not shared with the
// host or another pre-existing container, where an init-like process is
// already running.
// This does *NOT* modify the container command - that must be done elsewhere.
func addContainerInitBinary(s *specgen.SpecGenerator, path string) (spec.Mount, error) {
	mount := spec.Mount{
		Destination: define.ContainerInitPath,
		Type:        define.TypeBind,
		Source:      path,
		Options:     []string{define.TypeBind, "ro"},
	}

	if path == "" {
		return mount, errors.New("please specify a path to the container-init binary")
	}
	if !s.PidNS.IsPrivate() {
		return mount, errors.New("cannot add init binary as PID 1 (PID namespace isn't private)")
	}
	if s.Systemd == "always" {
		return mount, errors.New("cannot use container-init binary with systemd=always")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return mount, fmt.Errorf("container-init binary not found on the host: %w", err)
	}
	return mount, nil
}

// Supersede existing mounts in the spec with new, user-specified mounts.
// TODO: Should we unmount subtree mounts? E.g., if /tmp/ is mounted by
// one mount, and we already have /tmp/a and /tmp/b, should we remove
// the /tmp/a and /tmp/b mounts in favor of the more general /tmp?
func SupersedeUserMounts(mounts []spec.Mount, configMount []spec.Mount) []spec.Mount {
	if len(mounts) > 0 {
		// If we have overlappings mounts, remove them from the spec in favor of
		// the user-added volume mounts
		destinations := make(map[string]bool)
		for _, mount := range mounts {
			destinations[path.Clean(mount.Destination)] = true
		}
		// Copy all mounts from spec to defaultMounts, except for
		//  - mounts overridden by a user supplied mount;
		//  - all mounts under /dev if a user supplied /dev is present;
		mountDev := destinations["/dev"]
		for _, mount := range configMount {
			if _, ok := destinations[path.Clean(mount.Destination)]; !ok {
				if mountDev && strings.HasPrefix(mount.Destination, "/dev/") {
					// filter out everything under /dev if /dev is user-mounted
					continue
				}

				logrus.Debugf("Adding mount %s", mount.Destination)
				mounts = append(mounts, mount)
			}
		}
		return mounts
	}
	return configMount
}

func InitFSMounts(mounts []spec.Mount) error {
	for i, m := range mounts {
		switch {
		case m.Type == define.TypeBind:
			opts, err := util.ProcessOptions(m.Options, false, m.Source)
			if err != nil {
				return err
			}
			mounts[i].Options = opts
		case m.Type == define.TypeTmpfs && filepath.Clean(m.Destination) != "/dev":
			opts, err := util.ProcessOptions(m.Options, true, "")
			if err != nil {
				return err
			}
			mounts[i].Options = opts
		}
	}
	return nil
}
