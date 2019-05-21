package createconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage/pkg/stringid"
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
// Handles --volumes-from, --volumes, --tmpfs, --init, and --init-path flags.
// TODO: Named volume options -  should we default to rprivate? It bakes into a
// bind mount under the hood...
// TODO: handle options parsing/processing via containers/storage/pkg/mount
func (config *CreateConfig) parseVolumes(runtime *libpod.Runtime) ([]spec.Mount, []*libpod.ContainerNamedVolume, error) {
	// Add image volumes.
	baseMounts, baseVolumes, err := config.getImageVolumes()
	if err != nil {
		return nil, nil, err
	}

	// Add --volumes-from.
	// Overrides image volumes unconditionally.
	vFromMounts, vFromVolumes, err := config.getVolumesFrom(runtime)
	if err != nil {
		return nil, nil, err
	}
	for dest, mount := range vFromMounts {
		baseMounts[dest] = mount
	}
	for dest, volume := range vFromVolumes {
		baseVolumes[dest] = volume
	}

	// Next mounts from the --mounts flag.
	// Do not override yet.
	unifiedMounts, unifiedVolumes, err := config.getMounts()
	if err != nil {
		return nil, nil, err
	}

	// Next --volumes flag.
	// Do not override yet.
	volumeMounts, volumeVolumes, err := config.getVolumeMounts()
	if err != nil {
		return nil, nil, err
	}

	// Next --tmpfs flag.
	// Do not override yet.
	tmpfsMounts, err := config.getTmpfsMounts()
	if err != nil {
		return nil, nil, err
	}

	// Unify mounts from --mount, --volume, --tmpfs.
	// Also add mounts + volumes directly from createconfig.
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
	// Now spec mounts and volumes
	for _, mount := range config.Mounts {
		dest := mount.Destination
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedMounts[dest] = mount
	}
	for _, volume := range config.NamedVolumes {
		dest := volume.Dest
		if _, ok := unifiedVolumes[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedVolumes[dest] = volume
	}

	// If requested, add container init binary
	if config.Init {
		initPath := config.InitPath
		if initPath == "" {
			rtc, err := runtime.GetConfig()
			if err != nil {
				return nil, nil, err
			}
			initPath = rtc.InitPath
		}
		initMount, err := config.addContainerInitBinary(initPath)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := unifiedMounts[initMount.Destination]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, "conflict with mount added by --init to %q", initMount.Destination)
		}
		unifiedMounts[initMount.Destination] = initMount
	}

	// Before superceding, we need to find volume mounts which conflict with
	// named volumes, and vice versa.
	// We'll delete the conflicts here as we supercede.
	for dest := range unifiedMounts {
		if _, ok := baseVolumes[dest]; ok {
			delete(baseVolumes, dest)
		}
	}
	for dest := range unifiedVolumes {
		if _, ok := baseMounts[dest]; ok {
			delete(baseMounts, dest)
		}
	}

	// Supercede volumes-from/image volumes with unified volumes from above.
	// This is an unconditional replacement.
	for dest, mount := range unifiedMounts {
		baseMounts[dest] = mount
	}
	for dest, volume := range unifiedVolumes {
		baseVolumes[dest] = volume
	}

	// If requested, add tmpfs filesystems for read-only containers.
	// Need to keep track of which we created, so we don't modify options
	// for them later...
	readonlyTmpfs := map[string]bool{
		"/tmp":     false,
		"/var/tmp": false,
		"/run":     false,
	}
	if config.ReadOnlyRootfs && config.ReadOnlyTmpfs {
		options := []string{"rw", "rprivate", "nosuid", "nodev", "tmpcopyup", "size=65536k"}
		for dest := range readonlyTmpfs {
			if _, ok := baseMounts[dest]; ok {
				continue
			}
			localOpts := options
			if dest == "/run" {
				localOpts = append(localOpts, "noexec")
			}
			baseMounts[dest] = spec.Mount{
				Destination: dest,
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     localOpts,
			}
			readonlyTmpfs[dest] = true
		}
	}

	// Check for conflicts between named volumes and mounts
	for dest := range baseMounts {
		if _, ok := baseVolumes[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, "conflict at mount destination %v", dest)
		}
	}
	for dest := range baseVolumes {
		if _, ok := baseMounts[dest]; ok {
			return nil, nil, errors.Wrapf(errDuplicateDest, "conflict at mount destination %v", dest)
		}
	}

	// Final step: maps to arrays
	finalMounts := make([]spec.Mount, 0, len(baseMounts))
	for _, mount := range baseMounts {
		// All user-added tmpfs mounts need their options processed.
		// Exception: mounts added by the ReadOnlyTmpfs option, which
		// contain several exceptions to normal options rules.
		if mount.Type == TypeTmpfs && !readonlyTmpfs[mount.Destination] {
			opts, err := util.ProcessTmpfsOptions(mount.Options)
			if err != nil {
				return nil, nil, err
			}
			mount.Options = opts
		}
		finalMounts = append(finalMounts, mount)
	}
	finalVolumes := make([]*libpod.ContainerNamedVolume, 0, len(baseVolumes))
	for _, volume := range baseVolumes {
		finalVolumes = append(finalVolumes, volume)
	}

	logrus.Debugf("Got mounts: %v", finalMounts)
	logrus.Debugf("Got volumes: %v", finalVolumes)

	return finalMounts, finalVolumes, nil
}

// Parse volumes from - a set of containers whose volumes we will mount in.
// Grab the containers, retrieve any user-created spec mounts and all named
// volumes, and return a list of them.
// Conflicts are resolved simply - the last container specified wins.
// Container names may be suffixed by mount options after a colon.
func (config *CreateConfig) getVolumesFrom(runtime *libpod.Runtime) (map[string]spec.Mount, map[string]*libpod.ContainerNamedVolume, error) {
	// TODO: This can probably be disabled now
	if os.Geteuid() != 0 {
		return nil, nil, nil
	}

	// Both of these are maps of mount destination to mount type.
	// We ensure that each destination is only mounted to once in this way.
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*libpod.ContainerNamedVolume)

	for _, vol := range config.VolumesFrom {
		options := []string{}
		splitVol := strings.SplitN(vol, ":", 2)
		if len(splitVol) == 2 {
			if strings.Contains(splitVol[1], "Z") ||
				strings.Contains(splitVol[1], "private") ||
				strings.Contains(splitVol[1], "slave") ||
				strings.Contains(splitVol[1], "shared") {
				return nil, nil, errors.Errorf("invalid options %q, can only specify 'ro', 'rw', and 'z", splitVol[1])
			}
			options = strings.Split(splitVol[1], ",")
			if err := ValidateVolumeOpts(options); err != nil {
				return nil, nil, err
			}
		}
		ctr, err := runtime.LookupContainer(splitVol[0])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error looking up container %q for volumes-from", splitVol[0])
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
			return nil, nil, errors.Errorf("error retrieving container %s spec for volumes-from", ctr.ID())
		}
		for _, mnt := range spec.Mounts {
			if mnt.Type != TypeBind {
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
			finalNamedVolumes[namedVol.Dest] = namedVol
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

// getMounts takes user-provided input from the --mount flag and creates OCI
// spec mounts and Libpod named volumes.
// podman run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// podman run --mount type=tmpfs,target=/dev/shm ...
// podman run --mount type=volume,source=test-volume, ...
func (config *CreateConfig) getMounts() (map[string]spec.Mount, map[string]*libpod.ContainerNamedVolume, error) {
	finalMounts := make(map[string]spec.Mount)
	finalNamedVolumes := make(map[string]*libpod.ContainerNamedVolume)

	errInvalidSyntax := errors.Errorf("incorrect mount format: should be --mount type=<bind|tmpfs|volume>,[src=<host-dir|volume-name>,]target=<ctr-dir>[,options]")

	// TODO(vrothberg): the manual parsing can be replaced with a regular expression
	//                  to allow a more robust parsing of the mount format and to give
	//                  precise errors regarding supported format versus suppored options.
	for _, mount := range config.MountsFlag {
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
			return nil, nil, errors.Errorf("invalid fylesystem type %q", kv[1])
		}
	}

	return finalMounts, finalNamedVolumes, nil
}

// Parse a single bind mount entry from the --mount flag.
func getBindMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type: TypeBind,
	}

	setSource := false
	setDest := false

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "ro", "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options.
			// (Is this necessary?)
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
			if err := ValidateVolumeHostDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Source = kv[1]
			setSource = true
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = kv[1]
			setDest = true
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

	if err := ValidateVolumeOpts(newMount.Options); err != nil {
		return newMount, err
	}

	return newMount, nil
}

// Parse a single tmpfs mount entry from the --mount flag
func getTmpfsMount(args []string) (spec.Mount, error) {
	newMount := spec.Mount{
		Type:   TypeTmpfs,
		Source: TypeTmpfs,
	}

	setDest := false

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "ro", "nosuid", "nodev", "noexec":
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
			if err := ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = kv[1]
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
func getNamedVolume(args []string) (*libpod.ContainerNamedVolume, error) {
	newVolume := new(libpod.ContainerNamedVolume)

	setSource := false
	setDest := false

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "ro", "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options
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
			if err := ValidateVolumeCtrDir(kv[1]); err != nil {
				return nil, err
			}
			newVolume.Dest = kv[1]
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

// ValidateVolumeHostDir validates a volume mount's source directory
func ValidateVolumeHostDir(hostDir string) error {
	if len(hostDir) == 0 {
		return errors.Errorf("host directory cannot be empty")
	}
	if filepath.IsAbs(hostDir) {
		if _, err := os.Stat(hostDir); err != nil {
			return errors.Wrapf(err, "error checking path %q", hostDir)
		}
	}
	// If hostDir is not an absolute path, that means the user wants to create a
	// named volume. This will be done later on in the code.
	return nil
}

// ValidateVolumeCtrDir validates a volume mount's destination directory.
func ValidateVolumeCtrDir(ctrDir string) error {
	if len(ctrDir) == 0 {
		return errors.Errorf("container directory cannot be empty")
	}
	if !filepath.IsAbs(ctrDir) {
		return errors.Errorf("invalid container path %q, must be an absolute path", ctrDir)
	}
	return nil
}

// ValidateVolumeOpts validates a volume's options
func ValidateVolumeOpts(options []string) error {
	var foundRootPropagation, foundRWRO, foundLabelChange int
	for _, opt := range options {
		switch opt {
		case "rw", "ro":
			foundRWRO++
			if foundRWRO > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'rw' or 'ro' option", strings.Join(options, ", "))
			}
		case "z", "Z":
			foundLabelChange++
			if foundLabelChange > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'z' or 'Z' option", strings.Join(options, ", "))
			}
		case "private", "rprivate", "shared", "rshared", "slave", "rslave":
			foundRootPropagation++
			if foundRootPropagation > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 '[r]shared', '[r]private' or '[r]slave' option", strings.Join(options, ", "))
			}
		default:
			return errors.Errorf("invalid option type %q", opt)
		}
	}
	return nil
}

// GetVolumeMounts takes user provided input for bind mounts and creates Mount structs
func (config *CreateConfig) getVolumeMounts() (map[string]spec.Mount, map[string]*libpod.ContainerNamedVolume, error) {
	mounts := make(map[string]spec.Mount)
	volumes := make(map[string]*libpod.ContainerNamedVolume)

	volumeFormatErr := errors.Errorf("incorrect volume format, should be host-dir:ctr-dir[:option]")

	for _, vol := range config.Volumes {
		var (
			options []string
			src     string
			dest    string
		)

		splitVol := strings.Split(vol, ":")
		if len(splitVol) > 3 {
			return nil, nil, errors.Wrapf(volumeFormatErr, vol)
		}

		src = splitVol[0]
		if len(splitVol) == 1 {
			dest = src
		} else if len(splitVol) > 1 {
			dest = splitVol[1]
		}
		if len(splitVol) > 2 {
			options = strings.Split(splitVol[2], ",")
			if err := ValidateVolumeOpts(options); err != nil {
				return nil, nil, err
			}
		}

		if err := ValidateVolumeHostDir(src); err != nil {
			return nil, nil, err
		}
		if err := ValidateVolumeCtrDir(dest); err != nil {
			return nil, nil, err
		}

		if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
			// This is not a named volume
			newMount := spec.Mount{
				Destination: dest,
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
			newNamedVol := new(libpod.ContainerNamedVolume)
			newNamedVol.Name = src
			newNamedVol.Dest = dest
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

// Get mounts for container's image volumes
func (config *CreateConfig) getImageVolumes() (map[string]spec.Mount, map[string]*libpod.ContainerNamedVolume, error) {
	mounts := make(map[string]spec.Mount)
	volumes := make(map[string]*libpod.ContainerNamedVolume)

	if config.ImageVolumeType == "ignore" {
		return mounts, volumes, nil
	}

	for vol := range config.BuiltinImgVolumes {
		if config.ImageVolumeType == "tmpfs" {
			// Tmpfs image volumes are handled as mounts
			mount := spec.Mount{
				Destination: vol,
				Source:      TypeTmpfs,
				Type:        TypeTmpfs,
				Options:     []string{"rprivate", "rw", "nodev"},
			}
			mounts[vol] = mount
		} else {
			namedVolume := new(libpod.ContainerNamedVolume)
			namedVolume.Name = stringid.GenerateNonCryptoID()
			namedVolume.Options = []string{"rprivate", "rw", "nodev"}
			namedVolume.Dest = vol
			volumes[vol] = namedVolume
		}
	}

	return mounts, volumes, nil
}

// GetTmpfsMounts creates spec.Mount structs for user-requested tmpfs mounts
func (config *CreateConfig) getTmpfsMounts() (map[string]spec.Mount, error) {
	m := make(map[string]spec.Mount)
	for _, i := range config.Tmpfs {
		// Default options if nothing passed
		var options []string
		spliti := strings.Split(i, ":")
		destPath := spliti[0]
		if len(spliti) > 1 {
			options = strings.Split(spliti[1], ",")
		}

		if _, ok := m[destPath]; ok {
			return nil, errors.Wrapf(errDuplicateDest, destPath)
		}

		mount := spec.Mount{
			Destination: destPath,
			Type:        string(TypeTmpfs),
			Options:     options,
			Source:      string(TypeTmpfs),
		}
		m[destPath] = mount
	}
	return m, nil
}

// AddContainerInitBinary adds the init binary specified by path iff the
// container will run in a private PID namespace that is not shared with the
// host or another pre-existing container, where an init-like process is
// already running.
//
// Note that AddContainerInitBinary prepends "/dev/init" "--" to the command
// to execute the bind-mounted binary as PID 1.
func (config *CreateConfig) addContainerInitBinary(path string) (spec.Mount, error) {
	mount := spec.Mount{
		Destination: "/dev/init",
		Type:        TypeBind,
		Source:      path,
		Options:     []string{TypeBind, "ro"},
	}

	if path == "" {
		return mount, fmt.Errorf("please specify a path to the container-init binary")
	}
	if !config.PidMode.IsPrivate() {
		return mount, fmt.Errorf("cannot add init binary as PID 1 (PID namespace isn't private)")
	}
	if config.Systemd {
		return mount, fmt.Errorf("cannot use container-init binary with systemd")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return mount, errors.Wrap(err, "container-init binary not found on the host")
	}
	config.Command = append([]string{"/dev/init", "--"}, config.Command...)
	return mount, nil
}

// Supercede existing mounts in the spec with new, user-specified mounts.
// TODO: Should we unmount subtree mounts? E.g., if /tmp/ is mounted by
// one mount, and we already have /tmp/a and /tmp/b, should we remove
// the /tmp/a and /tmp/b mounts in favor of the more general /tmp?
func supercedeUserMounts(mounts []spec.Mount, configMount []spec.Mount) []spec.Mount {
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

// Ensure mount options on all mounts are correct
func initFSMounts(inputMounts []spec.Mount) []spec.Mount {
	var mounts []spec.Mount
	for _, m := range inputMounts {
		if m.Type == TypeBind {
			m.Options = util.ProcessOptions(m.Options)
		}
		if m.Type == TypeTmpfs {
			m.Options = append(m.Options, "tmpcopyup")
		}
		mounts = append(mounts, m)
	}
	return mounts
}
