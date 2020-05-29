package common

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
	// TypeVolume is the type for named volumes
	TypeVolume = "volume"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
)

var (
	errDuplicateDest = errors.Errorf("duplicate mount destination")
	optionArgError   = errors.Errorf("must provide an argument for option")
	noDestError      = errors.Errorf("must set volume destination")
)

// Parse all volume-related options in the create config into a set of mounts
// and named volumes to add to the container.
// Handles --volumes, --mount, and --tmpfs flags.
// Does not handle image volumes, init, and --volumes-from flags.
// Can also add tmpfs mounts from read-only tmpfs.
// TODO: handle options parsing/processing via containers/storage/pkg/mount
func parseVolumes(volumeFlag, mountFlag, tmpfsFlag []string, addReadOnlyTmpfs bool) ([]spec.Mount, []*specgen.NamedVolume, error) {
	// Get mounts from the --mounts flag.
	unifiedMounts, unifiedVolumes, err := getMounts(mountFlag)
	if err != nil {
		return nil, nil, err
	}

	// Next --volumes flag.
	volumeMounts, volumeVolumes, err := getVolumeMounts(volumeFlag)
	if err != nil {
		return nil, nil, err
	}

	// Next --tmpfs flag.
	tmpfsMounts, err := getTmpfsMounts(tmpfsFlag)
	if err != nil {
		return nil, nil, err
	}

	// Unify mounts from --mount, --volume, --tmpfs.
	// Start with --volume.
	for dest, mount := range volumeMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedMounts[dest] = mount
	}
	for dest, volume := range volumeVolumes {
		if _, ok := unifiedVolumes[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedVolumes[dest] = volume
	}
	// Now --tmpfs
	for dest, tmpfs := range tmpfsMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedMounts[dest] = tmpfs
	}

	// If requested, add tmpfs filesystems for read-only containers.
	if addReadOnlyTmpfs {
		readonlyTmpfs := []string{"/tmp", "/var/tmp", "/run"}
		options := []string{"rw", "rprivate", "nosuid", "nodev", "tmpcopyup"}
		for _, dest := range readonlyTmpfs {
			if _, ok := unifiedMounts[dest]; ok {
				continue
			}
			if _, ok := unifiedVolumes[dest]; ok {
				continue
			}
			localOpts := options
			if dest == "/run" {
				localOpts = append(localOpts, "noexec", "size=65536k")
			} else {
				localOpts = append(localOpts, "exec")
			}
			unifiedMounts[dest] = spec.Mount{
				Destination: dest,
				Type:        TypeTmpfs,
				Source:      "tmpfs",
				Options:     localOpts,
			}
		}
	}

	// Check for conflicts between named volumes and mounts
	for dest := range unifiedMounts {
		if _, ok := unifiedVolumes[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, "conflict at mount destination %v", dest)
		}
	}
	for dest := range unifiedVolumes {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, "conflict at mount destination %v", dest)
		}
	}

	// Final step: maps to arrays
	finalMounts := make([]spec.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		if mount.Type == TypeBind {
			absSrc, err := filepath.Abs(mount.Source)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "error getting absolute path of %s", mount.Source)
			}
			mount.Source = absSrc
		}
		finalMounts = append(finalMounts, mount)
	}
	finalVolumes := make([]*specgen.NamedVolume, 0, len(unifiedVolumes))
	for _, volume := range unifiedVolumes {
		finalVolumes = append(finalVolumes, volume)
	}

	return finalMounts, finalVolumes, nil
}

// getMounts takes user-provided input from the --mount flag and creates OCI
// spec mounts and Libpod named volumes.
// podman run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// podman run --mount type=tmpfs,target=/dev/shm ...
// podman run --mount type=volume,source=test-volume, ...
func getMounts(mountFlag []string) (map[string]spec.Mount, map[string]*specgen.NamedVolume, error) {
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*specgen.NamedVolume)

	errInvalidSyntax := errors.Errorf("incorrect mount format: should be --mount type=<bind|tmpfs|volume>,[src=<host-dir|volume-name>,]target=<ctr-dir>[,options]")

	// TODO(vrothberg): the manual parsing can be replaced with a regular expression
	//                  to allow a more robust parsing of the mount format and to give
	//                  precise errors regarding supported format versus supported options.
	for _, mount := range mountFlag {
		arr := strings.SplitN(mount, ",", 2)
		if len(arr) < 2 {
			return nil, nil, errors.Wrapf(errInvalidSyntax, "%q", mount)
		}
		kv := strings.Split(arr[0], "=")
		// TODO: type is not explicitly required in Docker.
		// If not specified, it defaults to "volume".
		if len(kv) != 2 || kv[0] != "type" {
			return nil, nil, errors.Wrapf(errInvalidSyntax, "%q", mount)
		}

		tokens := strings.Split(arr[1], ",")
		switch kv[1] {
		case TypeBind:
			mount, err := getBindMount(tokens)
			if err != nil {
				return nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, errors.Wrapf(errDuplicateDest, mount.Destination)
			}
			finalMounts[mount.Destination] = mount
		case TypeTmpfs:
			mount, err := getTmpfsMount(tokens)
			if err != nil {
				return nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, errors.Wrapf(errDuplicateDest, mount.Destination)
			}
			finalMounts[mount.Destination] = mount
		case "volume":
			volume, err := getNamedVolume(tokens)
			if err != nil {
				return nil, nil, err
			}
			if _, ok := finalNamedVolumes[volume.Dest]; ok {
				return nil, nil, errors.Wrapf(errDuplicateDest, volume.Dest)
			}
			finalNamedVolumes[volume.Dest] = volume
		default:
			return nil, nil, errors.Errorf("invalid filesystem type %q", kv[1])
		}
	}

	return finalMounts, finalNamedVolumes, nil
}

// Parse a single bind mount entry from the --mount flag.
func getBindMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type: TypeBind,
	}

	var setSource, setDest, setRORW, setSuid, setDev, setExec, setRelabel bool

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "bind-nonrecursive":
			newMount.Options = append(newMount.Options, "bind")
		case "readonly", "read-only":
			if setRORW {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'readonly', 'ro', or 'rw' options more than once")
			}
			setRORW = true
			switch len(kv) {
			case 1:
				newMount.Options = append(newMount.Options, "ro")
			case 2:
				switch strings.ToLower(kv[1]) {
				case "true":
					newMount.Options = append(newMount.Options, "ro")
				case "false":
					// RW is default, so do nothing
				default:
					return newMount, errors.Wrapf(optionArgError, "readonly must be set to true or false, instead received %q", kv[1])
				}
			default:
				return newMount, errors.Wrapf(optionArgError, "badly formatted option %q", val)
			}
		case "ro", "rw":
			if setRORW {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'readonly', 'ro', or 'rw' options more than once")
			}
			setRORW = true
			// Can be formatted as one of:
			// ro
			// ro=[true|false]
			// rw
			// rw=[true|false]
			switch len(kv) {
			case 1:
				newMount.Options = append(newMount.Options, kv[0])
			case 2:
				switch strings.ToLower(kv[1]) {
				case "true":
					newMount.Options = append(newMount.Options, kv[0])
				case "false":
					// Set the opposite only for rw
					// ro's opposite is the default
					if kv[0] == "rw" {
						newMount.Options = append(newMount.Options, "ro")
					}
				default:
					return newMount, errors.Wrapf(optionArgError, "%s must be set to true or false, instead received %q", kv[0], kv[1])
				}
			default:
				return newMount, errors.Wrapf(optionArgError, "badly formatted option %q", val)
			}
		case "nosuid", "suid":
			if setSuid {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'nosuid' and 'suid' options more than once")
			}
			setSuid = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'nodev' and 'dev' options more than once")
			}
			setDev = true
			newMount.Options = append(newMount.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'noexec' and 'exec' options more than once")
			}
			setExec = true
			newMount.Options = append(newMount.Options, kv[0])
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z":
			newMount.Options = append(newMount.Options, kv[0])
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "src", "source":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := parse.ValidateVolumeHostDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Source = kv[1]
			setSource = true
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = filepath.Clean(kv[1])
			setDest = true
		case "relabel":
			if setRelabel {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'relabel' option more than once")
			}
			setRelabel = true
			if len(kv) != 2 {
				return newMount, errors.Wrapf(util.ErrBadMntOption, "%s mount option must be 'private' or 'shared'", kv[0])
			}
			switch kv[1] {
			case "private":
				newMount.Options = append(newMount.Options, "z")
			case "shared":
				newMount.Options = append(newMount.Options, "Z")
			default:
				return newMount, errors.Wrapf(util.ErrBadMntOption, "%s mount option must be 'private' or 'shared'", kv[0])
			}
		default:
			return newMount, errors.Wrapf(util.ErrBadMntOption, kv[0])
		}
	}

	if !setDest {
		return newMount, noDestError
	}

	if !setSource {
		newMount.Source = newMount.Destination
	}

	options, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, err
	}
	newMount.Options = options
	return newMount, nil
}

// Parse a single tmpfs mount entry from the --mount flag
func getTmpfsMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type:   TypeTmpfs,
		Source: TypeTmpfs,
	}

	var setDest, setRORW, setSuid, setDev, setExec, setTmpcopyup bool

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "tmpcopyup", "notmpcopyup":
			if setTmpcopyup {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'tmpcopyup' and 'notmpcopyup' options more than once")
			}
			setTmpcopyup = true
			newMount.Options = append(newMount.Options, kv[0])
		case "ro", "rw":
			if setRORW {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'ro' and 'rw' options more than once")
			}
			setRORW = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nosuid", "suid":
			if setSuid {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'nosuid' and 'suid' options more than once")
			}
			setSuid = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'nodev' and 'dev' options more than once")
			}
			setDev = true
			newMount.Options = append(newMount.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return newMount, errors.Wrapf(optionArgError, "cannot pass 'noexec' and 'exec' options more than once")
			}
			setExec = true
			newMount.Options = append(newMount.Options, kv[0])
		case "tmpfs-mode":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("mode=%s", kv[1]))
		case "tmpfs-size":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("size=%s", kv[1]))
		case "src", "source":
			return newMount, errors.Errorf("source is not supported with tmpfs mounts")
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = filepath.Clean(kv[1])
			setDest = true
		default:
			return newMount, errors.Wrapf(util.ErrBadMntOption, kv[0])
		}
	}

	if !setDest {
		return newMount, noDestError
	}

	return newMount, nil
}

// Parse a single volume mount entry from the --mount flag.
// Note that the volume-label option for named volumes is currently NOT supported.
// TODO: add support for --volume-label
func getNamedVolume(args []string) (*specgen.NamedVolume, error) {
	newVolume := new(specgen.NamedVolume)

	var setSource, setDest, setRORW, setSuid, setDev, setExec bool

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "ro", "rw":
			if setRORW {
				return nil, errors.Wrapf(optionArgError, "cannot pass 'ro' and 'rw' options more than once")
			}
			setRORW = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "nosuid", "suid":
			if setSuid {
				return nil, errors.Wrapf(optionArgError, "cannot pass 'nosuid' and 'suid' options more than once")
			}
			setSuid = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return nil, errors.Wrapf(optionArgError, "cannot pass 'nodev' and 'dev' options more than once")
			}
			setDev = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return nil, errors.Wrapf(optionArgError, "cannot pass 'noexec' and 'exec' options more than once")
			}
			setExec = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "volume-label":
			return nil, errors.Errorf("the --volume-label option is not presently implemented")
		case "src", "source":
			if len(kv) == 1 {
				return nil, errors.Wrapf(optionArgError, kv[0])
			}
			newVolume.Name = kv[1]
			setSource = true
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return nil, errors.Wrapf(optionArgError, kv[0])
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return nil, err
			}
			newVolume.Dest = filepath.Clean(kv[1])
			setDest = true
		default:
			return nil, errors.Wrapf(util.ErrBadMntOption, kv[0])
		}
	}

	if !setSource {
		return nil, errors.Errorf("must set source volume")
	}
	if !setDest {
		return nil, noDestError
	}

	return newVolume, nil
}

func getVolumeMounts(volumeFlag []string) (map[string]spec.Mount, map[string]*specgen.NamedVolume, error) {
	mounts := make(map[string]spec.Mount)
	volumes := make(map[string]*specgen.NamedVolume)

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
			return nil, nil, errors.Wrapf(volumeFormatErr, vol)
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
				return nil, nil, err
			}
		}

		// Do not check source dir for anonymous volumes
		if len(splitVol) > 1 {
			if err := parse.ValidateVolumeHostDir(src); err != nil {
				return nil, nil, err
			}
		}
		if err := parse.ValidateVolumeCtrDir(dest); err != nil {
			return nil, nil, err
		}

		cleanDest := filepath.Clean(dest)

		if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
			// This is not a named volume
			newMount := spec.Mount{
				Destination: cleanDest,
				Type:        string(TypeBind),
				Source:      src,
				Options:     options,
			}
			if _, ok := mounts[newMount.Destination]; ok {
				return nil, nil, errors.Wrapf(errDuplicateDest, newMount.Destination)
			}
			mounts[newMount.Destination] = newMount
		} else {
			// This is a named volume
			newNamedVol := new(specgen.NamedVolume)
			newNamedVol.Name = src
			newNamedVol.Dest = cleanDest
			newNamedVol.Options = options

			if _, ok := volumes[newNamedVol.Dest]; ok {
				return nil, nil, errors.Wrapf(errDuplicateDest, newNamedVol.Dest)
			}
			volumes[newNamedVol.Dest] = newNamedVol
		}

		logrus.Debugf("User mount %s:%s options %v", src, dest, options)
	}

	return mounts, volumes, nil
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

		if _, ok := m[destPath]; ok {
			return nil, errors.Wrapf(errDuplicateDest, destPath)
		}

		mount := spec.Mount{
			Destination: filepath.Clean(destPath),
			Type:        string(TypeTmpfs),
			Options:     options,
			Source:      string(TypeTmpfs),
		}
		m[destPath] = mount
	}
	return m, nil
}
