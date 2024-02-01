package specgenutil

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/specgenutilexternal"
	"github.com/containers/podman/v5/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	errOptionArg = errors.New("must provide an argument for option")
	errNoDest    = errors.New("must set volume destination")
)

// Parse all volume-related options in the create config into a set of mounts
// and named volumes to add to the container.
// Handles --volumes, --mount, and --tmpfs flags.
// Does not handle image volumes, init, and --volumes-from flags.
// Can also add tmpfs mounts from read-only tmpfs.
// TODO: handle options parsing/processing via containers/storage/pkg/mount
func parseVolumes(rtc *config.Config, volumeFlag, mountFlag, tmpfsFlag []string) ([]spec.Mount, []*specgen.NamedVolume, []*specgen.OverlayVolume, []*specgen.ImageVolume, error) {
	// Get mounts from the --mounts flag.
	// TODO: The runtime config part of this needs to move into pkg/specgen/generate to avoid querying containers.conf on the client.
	unifiedMounts, unifiedVolumes, unifiedImageVolumes, err := Mounts(mountFlag, rtc.Mounts())
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Next --volumes flag.
	volumeMounts, volumeVolumes, overlayVolumes, err := specgen.GenVolumeMounts(volumeFlag)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Next --tmpfs flag.
	tmpfsMounts, err := getTmpfsMounts(tmpfsFlag)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Unify mounts from --mount, --volume, --tmpfs.
	// Start with --volume.
	for dest, mount := range volumeMounts {
		if vol, ok := unifiedMounts[dest]; ok {
			if mount.Source == vol.Source &&
				specgen.StringSlicesEqual(vol.Options, mount.Options) {
				continue
			}
			return nil, nil, nil, nil, fmt.Errorf("%v: %w", dest, specgen.ErrDuplicateDest)
		}
		unifiedMounts[dest] = mount
	}
	for dest, volume := range volumeVolumes {
		if vol, ok := unifiedVolumes[dest]; ok {
			if volume.Name == vol.Name &&
				specgen.StringSlicesEqual(vol.Options, volume.Options) {
				continue
			}
			return nil, nil, nil, nil, fmt.Errorf("%v: %w", dest, specgen.ErrDuplicateDest)
		}
		unifiedVolumes[dest] = volume
	}
	// Now --tmpfs
	for dest, tmpfs := range tmpfsMounts {
		if vol, ok := unifiedMounts[dest]; ok {
			if vol.Type != define.TypeTmpfs {
				return nil, nil, nil, nil, fmt.Errorf("%v: %w", dest, specgen.ErrDuplicateDest)
			}
			continue
		}
		unifiedMounts[dest] = tmpfs
	}

	// Check for conflicts between named volumes, overlay & image volumes,
	// and mounts
	allMounts := make(map[string]bool)
	testAndSet := func(dest string) error {
		if _, ok := allMounts[dest]; ok {
			return fmt.Errorf("%v: %w", dest, specgen.ErrDuplicateDest)
		}
		allMounts[dest] = true
		return nil
	}
	for dest := range unifiedMounts {
		if err := testAndSet(dest); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	for dest := range unifiedVolumes {
		if err := testAndSet(dest); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	for dest := range overlayVolumes {
		if err := testAndSet(dest); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	for dest := range unifiedImageVolumes {
		if err := testAndSet(dest); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	// Final step: maps to arrays
	finalMounts := make([]spec.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		if mount.Type == define.TypeBind {
			absSrc, err := specgen.ConvertWinMountPath(mount.Source)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("getting absolute path of %s: %w", mount.Source, err)
			}
			mount.Source = absSrc
		}
		finalMounts = append(finalMounts, mount)
	}
	finalVolumes := make([]*specgen.NamedVolume, 0, len(unifiedVolumes))
	for _, volume := range unifiedVolumes {
		finalVolumes = append(finalVolumes, volume)
	}
	finalOverlayVolume := make([]*specgen.OverlayVolume, 0)
	for _, volume := range overlayVolumes {
		finalOverlayVolume = append(finalOverlayVolume, volume)
	}
	finalImageVolumes := make([]*specgen.ImageVolume, 0, len(unifiedImageVolumes))
	for _, volume := range unifiedImageVolumes {
		finalImageVolumes = append(finalImageVolumes, volume)
	}

	return finalMounts, finalVolumes, finalOverlayVolume, finalImageVolumes, nil
}

// Mounts takes user-provided input from the --mount flag as well as Mounts
// specified in containers.conf and creates OCI spec mounts and Libpod named volumes.
// podman run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// podman run --mount type=tmpfs,target=/dev/shm ...
// podman run --mount type=volume,source=test-volume, ...
func Mounts(mountFlag []string, configMounts []string) (map[string]spec.Mount, map[string]*specgen.NamedVolume, map[string]*specgen.ImageVolume, error) {
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*specgen.NamedVolume)
	finalImageVolumes := make(map[string]*specgen.ImageVolume)
	parseMounts := func(mounts []string, ignoreDup bool) error {
		for _, mount := range mounts {
			// TODO: Docker defaults to "volume" if no mount type is specified.
			mountType, tokens, err := specgenutilexternal.FindMountType(mount)
			if err != nil {
				return err
			}
			switch mountType {
			case define.TypeBind:
				mount, err := getBindMount(tokens)
				if err != nil {
					return err
				}
				if _, ok := finalMounts[mount.Destination]; ok {
					if ignoreDup {
						continue
					}
					return fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
				}
				finalMounts[mount.Destination] = mount
			case "glob":
				mounts, err := getGlobMounts(tokens)
				if err != nil {
					return err
				}
				for _, mount := range mounts {
					if _, ok := finalMounts[mount.Destination]; ok {
						if ignoreDup {
							continue
						}
						return fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
					}
					finalMounts[mount.Destination] = mount
				}
			case define.TypeTmpfs, define.TypeRamfs:
				mount, err := parseMemoryMount(tokens, mountType)
				if err != nil {
					return err
				}
				if _, ok := finalMounts[mount.Destination]; ok {
					if ignoreDup {
						continue
					}
					return fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
				}
				finalMounts[mount.Destination] = mount
			case define.TypeDevpts:
				mount, err := getDevptsMount(tokens)
				if err != nil {
					return err
				}
				if _, ok := finalMounts[mount.Destination]; ok {
					if ignoreDup {
						continue
					}
					return fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
				}
				finalMounts[mount.Destination] = mount
			case "image":
				volume, err := getImageVolume(tokens)
				if err != nil {
					return err
				}
				if _, ok := finalImageVolumes[volume.Destination]; ok {
					if ignoreDup {
						continue
					}
					return fmt.Errorf("%v: %w", volume.Destination, specgen.ErrDuplicateDest)
				}
				finalImageVolumes[volume.Destination] = volume
			case "volume":
				volume, err := getNamedVolume(tokens)
				if err != nil {
					return err
				}
				if _, ok := finalNamedVolumes[volume.Dest]; ok {
					if ignoreDup {
						continue
					}
					return fmt.Errorf("%v: %w", volume.Dest, specgen.ErrDuplicateDest)
				}
				finalNamedVolumes[volume.Dest] = volume
			default:
				return fmt.Errorf("invalid filesystem type %q", mountType)
			}
		}
		return nil
	}

	// Parse mounts passed in from the user
	if err := parseMounts(mountFlag, false); err != nil {
		return nil, nil, nil, err
	}

	// If user specified a mount flag that conflicts with a containers.conf flag, then ignore
	// the duplicate. This means that the parsing of the containers.conf configMounts should always
	// happen second.
	if err := parseMounts(configMounts, true); err != nil {
		return nil, nil, nil, fmt.Errorf("parsing containers.conf mounts: %w", err)
	}

	return finalMounts, finalNamedVolumes, finalImageVolumes, nil
}

func parseMountOptions(mountType string, args []string) (*spec.Mount, error) {
	var setTmpcopyup, setRORW, setSuid, setDev, setExec, setRelabel, setOwnership, setSwap bool

	mnt := spec.Mount{}
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "bind-nonrecursive":
			if mountType != define.TypeBind {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			mnt.Options = append(mnt.Options, define.TypeBind)
		case "bind-propagation":
			if mountType != define.TypeBind {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			mnt.Options = append(mnt.Options, value)
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		case "idmap":
			if hasValue {
				mnt.Options = append(mnt.Options, fmt.Sprintf("idmap=%s", value))
			} else {
				mnt.Options = append(mnt.Options, "idmap")
			}
		case "readonly", "ro", "rw":
			if setRORW {
				return nil, fmt.Errorf("cannot pass 'readonly', 'ro', or 'rw' mnt.Options more than once: %w", errOptionArg)
			}
			setRORW = true
			// Can be formatted as one of:
			// readonly
			// readonly=[true|false]
			// ro
			// ro=[true|false]
			// rw
			// rw=[true|false]
			if name == "readonly" {
				name = "ro"
			}
			if hasValue {
				switch strings.ToLower(value) {
				case "true":
					mnt.Options = append(mnt.Options, name)
				case "false":
					// Set the opposite only for rw
					// ro's opposite is the default
					if name == "rw" {
						mnt.Options = append(mnt.Options, "ro")
					}
				}
			} else {
				mnt.Options = append(mnt.Options, name)
			}
		case "nodev", "dev":
			if setDev {
				return nil, fmt.Errorf("cannot pass 'nodev' and 'dev' mnt.Options more than once: %w", errOptionArg)
			}
			setDev = true
			mnt.Options = append(mnt.Options, name)
		case "noexec", "exec":
			if setExec {
				return nil, fmt.Errorf("cannot pass 'noexec' and 'exec' mnt.Options more than once: %w", errOptionArg)
			}
			setExec = true
			mnt.Options = append(mnt.Options, name)
		case "nosuid", "suid":
			if setSuid {
				return nil, fmt.Errorf("cannot pass 'nosuid' and 'suid' mnt.Options more than once: %w", errOptionArg)
			}
			setSuid = true
			mnt.Options = append(mnt.Options, name)
		case "noswap":
			if setSwap {
				return nil, fmt.Errorf("cannot pass 'noswap' mnt.Options more than once: %w", errOptionArg)
			}
			if rootless.IsRootless() {
				return nil, fmt.Errorf("the 'noswap' option is only allowed with rootful tmpfs mounts: %w", errOptionArg)
			}
			setSwap = true
			mnt.Options = append(mnt.Options, name)
		case "relabel":
			if setRelabel {
				return nil, fmt.Errorf("cannot pass 'relabel' option more than once: %w", errOptionArg)
			}
			setRelabel = true
			if !hasValue {
				return nil, fmt.Errorf("%s mount option must be 'private' or 'shared': %w", name, util.ErrBadMntOption)
			}
			switch value {
			case "private":
				mnt.Options = append(mnt.Options, "Z")
			case "shared":
				mnt.Options = append(mnt.Options, "z")
			default:
				return nil, fmt.Errorf("%s mount option must be 'private' or 'shared': %w", name, util.ErrBadMntOption)
			}
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "unbindable", "runbindable", "Z", "z", "no-dereference":
			mnt.Options = append(mnt.Options, name)
		case "src", "source":
			if mountType == define.TypeTmpfs {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			if mnt.Source != "" {
				return nil, fmt.Errorf("cannot pass %q option more than once: %w", name, errOptionArg)
			}
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			if len(value) == 0 {
				return nil, fmt.Errorf("host directory cannot be empty: %w", errOptionArg)
			}
			mnt.Source = value
		case "target", "dst", "destination":
			if mnt.Destination != "" {
				return nil, fmt.Errorf("cannot pass %q option more than once: %w", name, errOptionArg)
			}
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(value); err != nil {
				return nil, err
			}
			mnt.Destination = unixPathClean(value)
		case "tmpcopyup", "notmpcopyup":
			if mountType != define.TypeTmpfs {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			if setTmpcopyup {
				return nil, fmt.Errorf("cannot pass 'tmpcopyup' and 'notmpcopyup' mnt.Options more than once: %w", errOptionArg)
			}
			setTmpcopyup = true
			mnt.Options = append(mnt.Options, name)
		case "tmpfs-mode":
			if mountType != define.TypeTmpfs {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			mnt.Options = append(mnt.Options, fmt.Sprintf("mode=%s", value))
		case "tmpfs-size":
			if mountType != define.TypeTmpfs {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			mnt.Options = append(mnt.Options, fmt.Sprintf("size=%s", value))
		case "U", "chown":
			if setOwnership {
				return nil, fmt.Errorf("cannot pass 'U' or 'chown' option more than once: %w", errOptionArg)
			}
			ok, err := validChownFlag(value)
			if err != nil {
				return nil, err
			}
			if ok {
				mnt.Options = append(mnt.Options, "U")
			}
			setOwnership = true
		case "volume-label":
			if mountType != define.TypeVolume {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			return nil, fmt.Errorf("the --volume-label option is not presently implemented")
		case "volume-opt":
			if mountType != define.TypeVolume {
				return nil, fmt.Errorf("%q option not supported for %q mount types", name, mountType)
			}
			mnt.Options = append(mnt.Options, arg)
		default:
			return nil, fmt.Errorf("%s: %w", name, util.ErrBadMntOption)
		}
	}
	if mountType != "glob" && len(mnt.Destination) == 0 {
		return nil, errNoDest
	}
	return &mnt, nil
}

// Parse glob mounts entry from the --mount flag.
func getGlobMounts(args []string) ([]spec.Mount, error) {
	mounts := []spec.Mount{}

	mnt, err := parseMountOptions("glob", args)
	if err != nil {
		return nil, err
	}

	globs, err := filepath.Glob(mnt.Source)
	if err != nil {
		return nil, err
	}
	if len(globs) == 0 {
		return nil, fmt.Errorf("no file paths matching glob %q", mnt.Source)
	}

	options, err := parse.ValidateVolumeOpts(mnt.Options)
	if err != nil {
		return nil, err
	}
	for _, src := range globs {
		var newMount spec.Mount
		newMount.Type = define.TypeBind
		newMount.Options = options
		newMount.Source = src
		if len(mnt.Destination) == 0 {
			newMount.Destination = src
		} else {
			newMount.Destination = filepath.Join(mnt.Destination, filepath.Base(src))
		}
		mounts = append(mounts, newMount)
	}

	return mounts, nil
}

// Parse a single bind mount entry from the --mount flag.
func getBindMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type: define.TypeBind,
	}
	var err error
	mnt, err := parseMountOptions(newMount.Type, args)
	if err != nil {
		return newMount, err
	}

	if len(mnt.Destination) == 0 {
		return newMount, errNoDest
	}

	if len(mnt.Source) == 0 {
		mnt.Source = mnt.Destination
	}

	options, err := parse.ValidateVolumeOpts(mnt.Options)
	if err != nil {
		return newMount, err
	}
	newMount.Source = mnt.Source
	newMount.Destination = mnt.Destination
	newMount.Options = options
	return newMount, nil
}

// Parse a single tmpfs/ramfs mount entry from the --mount flag
func parseMemoryMount(args []string, mountType string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type:   mountType,
		Source: mountType,
	}

	var err error
	mnt, err := parseMountOptions(newMount.Type, args)
	if err != nil {
		return newMount, err
	}
	if len(mnt.Destination) == 0 {
		return newMount, errNoDest
	}
	newMount.Destination = mnt.Destination
	newMount.Options = mnt.Options
	return newMount, nil
}

// Parse a single devpts mount entry from the --mount flag
func getDevptsMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type:   define.TypeDevpts,
		Source: define.TypeDevpts,
	}

	var setDest bool

	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "uid", "gid", "mode", "ptxmode", "newinstance", "max":
			newMount.Options = append(newMount.Options, arg)
		case "target", "dst", "destination":
			if !hasValue {
				return newMount, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(value); err != nil {
				return newMount, err
			}
			newMount.Destination = unixPathClean(value)
			setDest = true
		default:
			return newMount, fmt.Errorf("%s: %w", name, util.ErrBadMntOption)
		}
	}

	if !setDest {
		return newMount, errNoDest
	}

	return newMount, nil
}

// Parse a single volume mount entry from the --mount flag.
// Note that the volume-label option for named volumes is currently NOT supported.
// TODO: add support for --volume-label
func getNamedVolume(args []string) (*specgen.NamedVolume, error) {
	newVolume := new(specgen.NamedVolume)

	mnt, err := parseMountOptions(define.TypeVolume, args)
	if err != nil {
		return nil, err
	}
	if len(mnt.Destination) == 0 {
		return nil, errNoDest
	}
	newVolume.Options = mnt.Options
	newVolume.Name = mnt.Source
	newVolume.Dest = mnt.Destination
	return newVolume, nil
}

// Parse the arguments into an image volume. An image volume is a volume based
// on a container image.  The container image is first mounted on the host and
// is then bind-mounted into the container.  An ImageVolume is always mounted
// read-only.
func getImageVolume(args []string) (*specgen.ImageVolume, error) {
	newVolume := new(specgen.ImageVolume)

	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "src", "source":
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			newVolume.Source = value
		case "target", "dst", "destination":
			if !hasValue {
				return nil, fmt.Errorf("%v: %w", name, errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(value); err != nil {
				return nil, err
			}
			newVolume.Destination = unixPathClean(value)
		case "rw", "readwrite":
			switch value {
			case "true":
				newVolume.ReadWrite = true
			case "false":
				// Nothing to do. RO is default.
			default:
				return nil, fmt.Errorf("invalid rw value %q: %w", value, util.ErrBadMntOption)
			}
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		default:
			return nil, fmt.Errorf("%s: %w", name, util.ErrBadMntOption)
		}
	}

	if len(newVolume.Source)*len(newVolume.Destination) == 0 {
		return nil, errors.New("must set source and destination for image volume")
	}

	return newVolume, nil
}

// GetTmpfsMounts creates spec.Mount structs for user-requested tmpfs mounts
func getTmpfsMounts(tmpfsFlag []string) (map[string]spec.Mount, error) {
	m := make(map[string]spec.Mount)
	for _, i := range tmpfsFlag {
		// Default options if nothing passed
		var options []string
		spliti := strings.Split(i, ":")
		destPath := spliti[0]
		if err := parse.ValidateVolumeCtrDir(spliti[0]); err != nil {
			return nil, err
		}
		if len(spliti) > 1 {
			options = strings.Split(spliti[1], ",")
		}

		if vol, ok := m[destPath]; ok {
			if specgen.StringSlicesEqual(vol.Options, options) {
				continue
			}
			return nil, fmt.Errorf("%v: %w", destPath, specgen.ErrDuplicateDest)
		}
		mount := spec.Mount{
			Destination: unixPathClean(destPath),
			Type:        define.TypeTmpfs,
			Options:     options,
			Source:      define.TypeTmpfs,
		}
		m[destPath] = mount
	}
	return m, nil
}

// validChownFlag ensures that the U or chown flag is correctly used
func validChownFlag(value string) (bool, error) {
	// U=[true|false]
	switch {
	case strings.EqualFold(value, "true"), value == "":
		return true, nil
	case strings.EqualFold(value, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("'U' or 'chown' must be set to true or false, instead received %q: %w", value, errOptionArg)
	}
}

// Use path instead of filepath to preserve Unix style paths on Windows
func unixPathClean(p string) string {
	return path.Clean(p)
}
