package specgenutil

import (
	"encoding/csv"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/containers/common/pkg/parse"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	errOptionArg     = errors.New("must provide an argument for option")
	errNoDest        = errors.New("must set volume destination")
	errInvalidSyntax = errors.New("incorrect mount format: should be --mount type=<bind|tmpfs|volume>,[src=<host-dir|volume-name>,]target=<ctr-dir>[,options]")
)

// Parse all volume-related options in the create config into a set of mounts
// and named volumes to add to the container.
// Handles --volumes, --mount, and --tmpfs flags.
// Does not handle image volumes, init, and --volumes-from flags.
// Can also add tmpfs mounts from read-only tmpfs.
// TODO: handle options parsing/processing via containers/storage/pkg/mount
func parseVolumes(volumeFlag, mountFlag, tmpfsFlag []string, addReadOnlyTmpfs bool) ([]spec.Mount, []*specgen.NamedVolume, []*specgen.OverlayVolume, []*specgen.ImageVolume, error) {
	// Get mounts from the --mounts flag.
	unifiedMounts, unifiedVolumes, unifiedImageVolumes, err := Mounts(mountFlag)
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
			unifiedMounts[dest] = spec.Mount{
				Destination: dest,
				Type:        define.TypeTmpfs,
				Source:      "tmpfs",
				Options:     options,
			}
		}
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

// findMountType parses the input and extracts the type of the mount type and
// the remaining non-type tokens.
func findMountType(input string) (mountType string, tokens []string, err error) {
	// Split by comma, iterate over the slice and look for
	// "type=$mountType". Everything else is appended to tokens.
	found := false
	csvReader := csv.NewReader(strings.NewReader(input))
	records, err := csvReader.ReadAll()
	if err != nil {
		return "", nil, err
	}
	if len(records) != 1 {
		return "", nil, errInvalidSyntax
	}
	for _, s := range records[0] {
		kv := strings.Split(s, "=")
		if found || !(len(kv) == 2 && kv[0] == "type") {
			tokens = append(tokens, s)
			continue
		}
		mountType = kv[1]
		found = true
	}
	if !found {
		err = errInvalidSyntax
	}
	return
}

// Mounts takes user-provided input from the --mount flag and creates OCI
// spec mounts and Libpod named volumes.
// podman run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// podman run --mount type=tmpfs,target=/dev/shm ...
// podman run --mount type=volume,source=test-volume, ...
func Mounts(mountFlag []string) (map[string]spec.Mount, map[string]*specgen.NamedVolume, map[string]*specgen.ImageVolume, error) {
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*specgen.NamedVolume)
	finalImageVolumes := make(map[string]*specgen.ImageVolume)

	for _, mount := range mountFlag {
		// TODO: Docker defaults to "volume" if no mount type is specified.
		mountType, tokens, err := findMountType(mount)
		if err != nil {
			return nil, nil, nil, err
		}
		switch mountType {
		case define.TypeBind:
			mount, err := getBindMount(tokens)
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case define.TypeTmpfs:
			mount, err := getTmpfsMount(tokens)
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case define.TypeDevpts:
			mount, err := getDevptsMount(tokens)
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, specgen.ErrDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case "image":
			volume, err := getImageVolume(tokens)
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := finalImageVolumes[volume.Destination]; ok {
				return nil, nil, nil, fmt.Errorf("%v: %w", volume.Destination, specgen.ErrDuplicateDest)
			}
			finalImageVolumes[volume.Destination] = volume
		case "volume":
			volume, err := getNamedVolume(tokens)
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := finalNamedVolumes[volume.Dest]; ok {
				return nil, nil, nil, fmt.Errorf("%v: %w", volume.Dest, specgen.ErrDuplicateDest)
			}
			finalNamedVolumes[volume.Dest] = volume
		default:
			return nil, nil, nil, fmt.Errorf("invalid filesystem type %q", mountType)
		}
	}

	return finalMounts, finalNamedVolumes, finalImageVolumes, nil
}

// Parse a single bind mount entry from the --mount flag.
func getBindMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type: define.TypeBind,
	}

	var setSource, setDest, setRORW, setSuid, setDev, setExec, setRelabel, setOwnership bool

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "bind-nonrecursive":
			newMount.Options = append(newMount.Options, "bind")
		case "readonly", "ro", "rw":
			if setRORW {
				return newMount, fmt.Errorf("cannot pass 'readonly', 'ro', or 'rw' options more than once: %w", errOptionArg)
			}
			setRORW = true
			// Can be formatted as one of:
			// readonly
			// readonly=[true|false]
			// ro
			// ro=[true|false]
			// rw
			// rw=[true|false]
			if kv[0] == "readonly" {
				kv[0] = "ro"
			}
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
					return newMount, fmt.Errorf("'readonly', 'ro', or 'rw' must be set to true or false, instead received %q: %w", kv[1], errOptionArg)
				}
			default:
				return newMount, fmt.Errorf("badly formatted option %q: %w", val, errOptionArg)
			}
		case "nosuid", "suid":
			if setSuid {
				return newMount, fmt.Errorf("cannot pass 'nosuid' and 'suid' options more than once: %w", errOptionArg)
			}
			setSuid = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return newMount, fmt.Errorf("cannot pass 'nodev' and 'dev' options more than once: %w", errOptionArg)
			}
			setDev = true
			newMount.Options = append(newMount.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return newMount, fmt.Errorf("cannot pass 'noexec' and 'exec' options more than once: %w", errOptionArg)
			}
			setExec = true
			newMount.Options = append(newMount.Options, kv[0])
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "unbindable", "runbindable", "Z", "z":
			newMount.Options = append(newMount.Options, kv[0])
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "src", "source":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if len(kv[1]) == 0 {
				return newMount, fmt.Errorf("host directory cannot be empty: %w", errOptionArg)
			}
			newMount.Source = kv[1]
			setSource = true
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = unixPathClean(kv[1])
			setDest = true
		case "relabel":
			if setRelabel {
				return newMount, fmt.Errorf("cannot pass 'relabel' option more than once: %w", errOptionArg)
			}
			setRelabel = true
			if len(kv) != 2 {
				return newMount, fmt.Errorf("%s mount option must be 'private' or 'shared': %w", kv[0], util.ErrBadMntOption)
			}
			switch kv[1] {
			case "private":
				newMount.Options = append(newMount.Options, "Z")
			case "shared":
				newMount.Options = append(newMount.Options, "z")
			default:
				return newMount, fmt.Errorf("%s mount option must be 'private' or 'shared': %w", kv[0], util.ErrBadMntOption)
			}
		case "U", "chown":
			if setOwnership {
				return newMount, fmt.Errorf("cannot pass 'U' or 'chown' option more than once: %w", errOptionArg)
			}
			ok, err := validChownFlag(val)
			if err != nil {
				return newMount, err
			}
			if ok {
				newMount.Options = append(newMount.Options, "U")
			}
			setOwnership = true
		case "idmap":
			if len(kv) > 1 {
				newMount.Options = append(newMount.Options, fmt.Sprintf("idmap=%s", kv[1]))
			} else {
				newMount.Options = append(newMount.Options, "idmap")
			}
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		default:
			return newMount, fmt.Errorf("%s: %w", kv[0], util.ErrBadMntOption)
		}
	}

	if !setDest {
		return newMount, errNoDest
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
		Type:   define.TypeTmpfs,
		Source: define.TypeTmpfs,
	}

	var setDest, setRORW, setSuid, setDev, setExec, setTmpcopyup, setOwnership bool

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "tmpcopyup", "notmpcopyup":
			if setTmpcopyup {
				return newMount, fmt.Errorf("cannot pass 'tmpcopyup' and 'notmpcopyup' options more than once: %w", errOptionArg)
			}
			setTmpcopyup = true
			newMount.Options = append(newMount.Options, kv[0])
		case "ro", "rw":
			if setRORW {
				return newMount, fmt.Errorf("cannot pass 'ro' and 'rw' options more than once: %w", errOptionArg)
			}
			setRORW = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nosuid", "suid":
			if setSuid {
				return newMount, fmt.Errorf("cannot pass 'nosuid' and 'suid' options more than once: %w", errOptionArg)
			}
			setSuid = true
			newMount.Options = append(newMount.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return newMount, fmt.Errorf("cannot pass 'nodev' and 'dev' options more than once: %w", errOptionArg)
			}
			setDev = true
			newMount.Options = append(newMount.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return newMount, fmt.Errorf("cannot pass 'noexec' and 'exec' options more than once: %w", errOptionArg)
			}
			setExec = true
			newMount.Options = append(newMount.Options, kv[0])
		case "tmpfs-mode":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("mode=%s", kv[1]))
		case "tmpfs-size":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("size=%s", kv[1]))
		case "src", "source":
			return newMount, fmt.Errorf("source is not supported with tmpfs mounts")
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = unixPathClean(kv[1])
			setDest = true
		case "U", "chown":
			if setOwnership {
				return newMount, fmt.Errorf("cannot pass 'U' or 'chown' option more than once: %w", errOptionArg)
			}
			ok, err := validChownFlag(val)
			if err != nil {
				return newMount, err
			}
			if ok {
				newMount.Options = append(newMount.Options, "U")
			}
			setOwnership = true
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		default:
			return newMount, fmt.Errorf("%s: %w", kv[0], util.ErrBadMntOption)
		}
	}

	if !setDest {
		return newMount, errNoDest
	}

	return newMount, nil
}

// Parse a single devpts mount entry from the --mount flag
func getDevptsMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type:   define.TypeDevpts,
		Source: define.TypeDevpts,
	}

	var setDest bool

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "uid", "gid", "mode", "ptxmode", "newinstance", "max":
			newMount.Options = append(newMount.Options, val)
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = unixPathClean(kv[1])
			setDest = true
		default:
			return newMount, fmt.Errorf("%s: %w", kv[0], util.ErrBadMntOption)
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

	var setDest, setRORW, setSuid, setDev, setExec, setOwnership bool

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "volume-opt":
			newVolume.Options = append(newVolume.Options, val)
		case "ro", "rw":
			if setRORW {
				return nil, fmt.Errorf("cannot pass 'ro' and 'rw' options more than once: %w", errOptionArg)
			}
			setRORW = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "nosuid", "suid":
			if setSuid {
				return nil, fmt.Errorf("cannot pass 'nosuid' and 'suid' options more than once: %w", errOptionArg)
			}
			setSuid = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "nodev", "dev":
			if setDev {
				return nil, fmt.Errorf("cannot pass 'nodev' and 'dev' options more than once: %w", errOptionArg)
			}
			setDev = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "noexec", "exec":
			if setExec {
				return nil, fmt.Errorf("cannot pass 'noexec' and 'exec' options more than once: %w", errOptionArg)
			}
			setExec = true
			newVolume.Options = append(newVolume.Options, kv[0])
		case "volume-label":
			return nil, fmt.Errorf("the --volume-label option is not presently implemented")
		case "src", "source":
			if len(kv) == 1 {
				return nil, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			newVolume.Name = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return nil, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return nil, err
			}
			newVolume.Dest = unixPathClean(kv[1])
			setDest = true
		case "U", "chown":
			if setOwnership {
				return newVolume, fmt.Errorf("cannot pass 'U' or 'chown' option more than once: %w", errOptionArg)
			}
			ok, err := validChownFlag(val)
			if err != nil {
				return newVolume, err
			}
			if ok {
				newVolume.Options = append(newVolume.Options, "U")
			}
			setOwnership = true
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		default:
			return nil, fmt.Errorf("%s: %w", kv[0], util.ErrBadMntOption)
		}
	}

	if !setDest {
		return nil, errNoDest
	}

	return newVolume, nil
}

// Parse the arguments into an image volume. An image volume is a volume based
// on a container image.  The container image is first mounted on the host and
// is then bind-mounted into the container.  An ImageVolume is always mounted
// read-only.
func getImageVolume(args []string) (*specgen.ImageVolume, error) {
	newVolume := new(specgen.ImageVolume)

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "src", "source":
			if len(kv) == 1 {
				return nil, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			newVolume.Source = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return nil, fmt.Errorf("%v: %w", kv[0], errOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return nil, err
			}
			newVolume.Destination = unixPathClean(kv[1])
		case "rw", "readwrite":
			switch kv[1] {
			case "true":
				newVolume.ReadWrite = true
			case "false":
				// Nothing to do. RO is default.
			default:
				return nil, fmt.Errorf("invalid rw value %q: %w", kv[1], util.ErrBadMntOption)
			}
		case "consistency":
			// Often used on MACs and mistakenly on Linux platforms.
			// Since Docker ignores this option so shall we.
			continue
		default:
			return nil, fmt.Errorf("%s: %w", kv[0], util.ErrBadMntOption)
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
func validChownFlag(flag string) (bool, error) {
	kv := strings.SplitN(flag, "=", 2)
	switch len(kv) {
	case 1:
	case 2:
		// U=[true|false]
		switch strings.ToLower(kv[1]) {
		case "true":
		case "false":
			return false, nil
		default:
			return false, fmt.Errorf("'U' or 'chown' must be set to true or false, instead received %q: %w", kv[1], errOptionArg)
		}
	default:
		return false, fmt.Errorf("badly formatted option %q: %w", flag, errOptionArg)
	}

	return true, nil
}

// Use path instead of filepath to preserve Unix style paths on Windows
func unixPathClean(p string) string {
	return path.Clean(p)
}
