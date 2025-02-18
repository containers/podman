package parse

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"errors"

	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/internal"
	internalUtil "github.com/containers/buildah/internal/util"
	internalVolumes "github.com/containers/buildah/internal/volumes"
	"github.com/containers/buildah/pkg/overlay"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/unshare"
	"github.com/containers/storage/pkg/mount"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
)

const (
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
	// TypeCache is the type for mounting a common persistent cache from host
	TypeCache = "cache"
	// mount=type=cache must create a persistent directory on host so its available for all consecutive builds.
	// Lifecycle of following directory will be inherited from how host machine treats temporary directory
	BuildahCacheDir = "buildah-cache"
	// mount=type=cache allows users to lock a cache store while its being used by another build
	BuildahCacheLockfile = "buildah-cache-lockfile"
	// All the lockfiles are stored in a separate directory inside `BuildahCacheDir`
	// Example `/var/tmp/buildah-cache/<target>/buildah-cache-lockfile`
	BuildahCacheLockfileDir = "buildah-cache-lockfiles"
)

var (
	errBadMntOption  = errors.New("invalid mount option")
	errBadOptionArg  = errors.New("must provide an argument for option")
	errBadVolDest    = errors.New("must set volume destination")
	errBadVolSrc     = errors.New("must set volume source")
	errDuplicateDest = errors.New("duplicate mount destination")
)

// GetBindMount parses a single bind mount entry from the --mount flag.
//
// Returns a Mount to add to the runtime spec's list of mounts, the ID of the
// image we mounted if we mounted one, the path of a mounted location if one
// needs to be unmounted and removed, and the path of an overlay mount if one
// needs to be cleaned up, or an error.
//
// The caller is expected to, after the command which uses the mount exits,
// clean up the overlay filesystem (if we provided a path to it), unmount and
// remove the mountpoint for the mounted filesystem (if we provided the path to
// its mountpoint), and then unmount the image (if we mounted one).
func GetBindMount(sys *types.SystemContext, args []string, contextDir string, store storage.Store, mountLabel string, additionalMountPoints map[string]internal.StageMountDetails, workDir, tmpDir string) (specs.Mount, string, string, string, error) {
	newMount := specs.Mount{
		Type: define.TypeBind,
	}

	mountReadability := ""
	setDest := ""
	bindNonRecursive := false
	fromImage := ""

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "type":
			// This is already processed
			continue
		case "bind-nonrecursive":
			newMount.Options = append(newMount.Options, "bind")
			bindNonRecursive = true
		case "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options.
			// (Is this necessary?)
			newMount.Options = append(newMount.Options, kv[0])
		case "rw", "readwrite":
			newMount.Options = append(newMount.Options, "rw")
			mountReadability = "rw"
		case "ro", "readonly":
			newMount.Options = append(newMount.Options, "ro")
			mountReadability = "ro"
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z", "U":
			newMount.Options = append(newMount.Options, kv[0])
		case "from":
			if len(kv) == 1 {
				return newMount, "", "", "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			fromImage = kv[1]
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, "", "", "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			switch kv[1] {
				default:
					return newMount, "", "", "", fmt.Errorf("%v: %q: %w", kv[0], kv[1], errBadMntOption)
				case "shared", "rshared", "private", "rprivate", "slave", "rslave":
				// this should be the relevant parts of the same list of options we accepted above
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "src", "source":
			if len(kv) == 1 {
				return newMount, "", "", "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Source = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, "", "", "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			targetPath := kv[1]
			setDest = targetPath
			if !path.IsAbs(targetPath) {
				targetPath = filepath.Join(workDir, targetPath)
			}
			if err := parse.ValidateVolumeCtrDir(targetPath); err != nil {
				return newMount, "", "", "", err
			}
			newMount.Destination = targetPath
		case "consistency":
			// Option for OS X only, has no meaning on other platforms
			// and can thus be safely ignored.
			// See also the handling of the equivalent "delegated" and "cached" in ValidateVolumeOpts
		default:
			return newMount, "", "", "", fmt.Errorf("%v: %w", kv[0], errBadMntOption)
		}
	}

	// default mount readability is always readonly
	if mountReadability == "" {
		newMount.Options = append(newMount.Options, "ro")
	}

	// Following variable ensures that we return imagename only if we did additional mount
	succeeded := false
	mountedImage := ""
	if fromImage != "" {
		mountPoint := ""
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[fromImage]; ok {
				mountPoint = val.MountPoint
			}
		}
		// if mountPoint of image was not found in additionalMap
		// or additionalMap was nil, try mounting image
		if mountPoint == "" {
			image, err := internalUtil.LookupImage(sys, store, fromImage)
			if err != nil {
				return newMount, "", "", "", err
			}

			mountPoint, err = image.Mount(context.Background(), nil, mountLabel)
			if err != nil {
				return newMount, "", "", "", err
			}
			mountedImage = image.ID()
			defer func() {
				if !succeeded {
					if _, err := store.UnmountImage(mountedImage, false); err != nil {
						logrus.Debugf("unmounting bind-mounted image %q: %v", fromImage, err)
					}
				}
			}()
		}
		contextDir = mountPoint
	}

	// buildkit parity: default bind option must be `rbind`
	// unless specified
	if !bindNonRecursive {
		newMount.Options = append(newMount.Options, "rbind")
	}

	if setDest == "" {
		return newMount, "", "", "", errBadVolDest
	}

	// buildkit parity: support absolute path for sources from current build context
	if contextDir != "" {
		// path should be /contextDir/specified path
                evaluated, err := copier.Eval(contextDir, contextDir+string(filepath.Separator)+newMount.Source, copier.EvalOptions{})
                if err != nil {
                    return newMount, "", "", "", err
                }
                newMount.Source = evaluated
	} else {
		// looks like its coming from `build run --mount=type=bind` allow using absolute path
		// error out if no source is set
		if newMount.Source == "" {
			return newMount, "", "", "", errBadVolSrc
		}
		if err := parse.ValidateVolumeHostDir(newMount.Source); err != nil {
			return newMount, "", "", "", err
		}
	}

	opts, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, "", "", "", err
	}
	newMount.Options = opts

	var intermediateMount string
	if contextDir != "" && newMount.Source != contextDir {
		rel, err := filepath.Rel(contextDir, newMount.Source)
		if err != nil {
			return newMount, "", "", "", fmt.Errorf("computing pathname of bind subdirectory: %w", err)
		}
		if rel != "." && rel != "/" {
			mnt, err := internalVolumes.BindFromChroot(contextDir, rel, tmpDir)
			if err != nil {
				return newMount, "", "", "", fmt.Errorf("sanitizing bind subdirectory %q: %w", newMount.Source, err)
			}
			logrus.Debugf("bind-mounted %q under %q to %q", rel, contextDir, mnt)
			intermediateMount = mnt
			newMount.Source = intermediateMount
		}
	}

	overlayDir := ""
	if mountedImage != "" || mountIsReadWrite(newMount) {
		if newMount, overlayDir, err = convertToOverlay(newMount, store, mountLabel, tmpDir, 0, 0); err != nil {
			return newMount, "", "", "", err
		}
	}

	succeeded = true

	return newMount, mountedImage, intermediateMount, overlayDir, nil
}

// CleanCacheMount gets the cache parent created by `--mount=type=cache` and removes it.
func CleanCacheMount() error {
	cacheParent := filepath.Join(internalUtil.GetTempDir(), BuildahCacheDir+"-"+strconv.Itoa(unshare.GetRootlessUID()))
	return os.RemoveAll(cacheParent)
}

func mountIsReadWrite(m specs.Mount) bool {
	// in case of conflicts, the last one wins, so it's not enough
	// to check for the presence of either "rw" or "ro" anywhere
	// with e.g. slices.Contains()
	rw := true
	for _, option := range m.Options {
		switch option {
		case "rw":
			rw = true
		case "ro":
			rw = false
		}
	}
	return rw
}

func convertToOverlay(m specs.Mount, store storage.Store, mountLabel, tmpDir string, uid, gid int) (specs.Mount, string, error) {
	overlayDir, err := overlay.TempDir(tmpDir, uid, gid)
	if err != nil {
		return specs.Mount{}, "", fmt.Errorf("setting up overlay for %q: %w", m.Destination, err)
	}
	graphOptions := store.GraphOptions()
	graphOptsCopy := make([]string, len(graphOptions))
	copy(graphOptsCopy, graphOptions)
	options := overlay.Options{GraphOpts: graphOptsCopy, ForceMount: true, MountLabel: mountLabel}
	fileInfo, err := os.Stat(m.Source)
	if err != nil {
		return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", m.Source, err)
	}
	// we might be trying to "overlay" for a non-directory, and the kernel doesn't like that very much
	var mountThisInstead specs.Mount
	if fileInfo.IsDir() {
		// do the normal thing of mounting this directory as a lower with a temporary upper
		mountThisInstead, err = overlay.MountWithOptions(overlayDir, m.Source, m.Destination, &options)
		if err != nil {
			return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", m.Source, err)
		}
	} else {
		// mount the parent directory as the lower with a temporary upper, and return a
		// bind mount from the non-directory in the merged directory to the destination
		sourceDir := filepath.Dir(m.Source)
		sourceBase := filepath.Base(m.Source)
		destination := m.Destination
		mountedOverlay, err := overlay.MountWithOptions(overlayDir, sourceDir, destination, &options)
		if err != nil {
			return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", sourceDir, err)
		}
		if mountedOverlay.Type != define.TypeBind {
			if err2 := overlay.RemoveTemp(overlayDir); err2 != nil {
				return specs.Mount{}, "", fmt.Errorf("cleaning up after failing to set up overlay: %v, while setting up overlay for %q: %w", err2, destination, err)
			}
			return specs.Mount{}, "", fmt.Errorf("setting up overlay for %q at %q: %w", mountedOverlay.Source, destination, err)
		}
		mountThisInstead = mountedOverlay
		mountThisInstead.Source = filepath.Join(mountedOverlay.Source, sourceBase)
		mountThisInstead.Destination = destination
	}
	return mountThisInstead, overlayDir, nil
}


// GetCacheMount parses a single cache mount entry from the --mount flag.
//
// Returns a Mount to add to the runtime spec's list of mounts, the path of a
// mounted filesystem if one needs to be unmounted, and an optional lock that
// needs to be released, or an error.
//
// The caller is expected to, after the command which uses the mount exits,
// unmount and remove the mountpoint of the mounted filesystem (if we provided
// the path to its mountpoint).
func GetCacheMount(args []string, additionalMountPoints map[string]internal.StageMountDetails, workDir, tmpDir string) (specs.Mount, string, *lockfile.LockFile, error) {
	var err error
	var mode uint64
	var buildahLockFilesDir string
	var (
		setDest           bool
		setShared         bool
		setReadOnly       bool
		foundSElinuxLabel bool
	)
	fromStage := ""
	newMount := specs.Mount{
		Type: define.TypeBind,
	}
	// if id is set a new subdirectory with `id` will be created under /host-temp/buildah-build-cache/id
	id := ""
	//buidkit parity: cache directory defaults to 755
	mode = 0o755
	//buidkit parity: cache directory defaults to uid 0 if not specified
	uid := 0
	//buidkit parity: cache directory defaults to gid 0 if not specified
	gid := 0
	// sharing mode
	sharing := "shared"

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "type":
			// This is already processed
			continue
		case "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options.
			// (Is this necessary?)
			newMount.Options = append(newMount.Options, kv[0])
		case "rw", "readwrite":
			newMount.Options = append(newMount.Options, "rw")
		case "readonly", "ro":
			// Alias for "ro"
			newMount.Options = append(newMount.Options, "ro")
			setReadOnly = true
		case "Z", "z":
			newMount.Options = append(newMount.Options, kv[0])
			foundSElinuxLabel = true
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "U":
			newMount.Options = append(newMount.Options, kv[0])
			setShared = true
		case "sharing":
			sharing = kv[1]
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			switch kv[1] {
				default:
					return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadMntOption)
				case "shared", "rshared", "private", "rprivate", "slave", "rslave":
				// this should be the relevant parts of the same list of options we accepted above
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "id":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			id = kv[1]
		case "from":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			fromStage = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			targetPath := kv[1]
			if !path.IsAbs(targetPath) {
				targetPath = filepath.Join(workDir, targetPath)
			}
			if err := parse.ValidateVolumeCtrDir(targetPath); err != nil {
				return newMount, "", nil, err
			}
			newMount.Destination = targetPath
			setDest = true
		case "src", "source":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Source = kv[1]
		case "mode":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			mode, err = strconv.ParseUint(kv[1], 8, 32)
			if err != nil {
				return newMount, "", nil, fmt.Errorf("unable to parse cache mode: %w", err)
			}
		case "uid":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			uid, err = strconv.Atoi(kv[1])
			if err != nil {
				return newMount, "", nil, fmt.Errorf("unable to parse cache uid: %w", err)
			}
		case "gid":
			if len(kv) == 1 {
				return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			gid, err = strconv.Atoi(kv[1])
			if err != nil {
				return newMount, "", nil, fmt.Errorf("unable to parse cache gid: %w", err)
			}
		default:
			return newMount, "", nil, fmt.Errorf("%v: %w", kv[0], errBadMntOption)
		}
	}

	// If selinux is enabled and no selinux option was configured
	// default to `z` i.e shared content label.
	if !foundSElinuxLabel && (selinux.EnforceMode() != selinux.Disabled) && fromStage == "" {
		newMount.Options = append(newMount.Options, "z")
	}

	if !setDest {
		return newMount, "", nil, errBadVolDest
	}

	thisCacheRoot := ""
	if fromStage != "" {
		// do not create and use a cache directory on the host,
		// instead use the location in the mounted stage or
		// temporary directory as the cache
		mountPoint := ""
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[fromStage]; ok {
				if !val.IsImage {
					mountPoint = val.MountPoint
				}
			}
		}
		// Cache does not supports using image so if not stage found
		// return with error
		if mountPoint == "" {
			return newMount, "", nil, fmt.Errorf("no stage or additional build context found with name %s", fromStage)
		}
		thisCacheRoot = mountPoint
	} else {
		// we need to create cache on host if no image is being used

		// since type is cache and cache can be reused by consecutive builds
		// create a common cache directory, which persists on hosts within temp lifecycle
		// add subdirectory if specified

		// cache parent directory: creates separate cache parent for each user.
		cacheParent := filepath.Join(internalUtil.GetTempDir(), BuildahCacheDir+"-"+strconv.Itoa(unshare.GetRootlessUID()))
		// create cache on host if not present
		err = os.MkdirAll(cacheParent, os.FileMode(0755))
		if err != nil {
			return newMount, "", nil, fmt.Errorf("unable to create build cache directory: %w", err)
		}

		if id != "" {
			// Don't let the user control where we place the directory.
			dirID := digest.FromString(id).Encoded()[:16]
			thisCacheRoot = filepath.Join(cacheParent, dirID)
			buildahLockFilesDir = filepath.Join(BuildahCacheLockfileDir, dirID)
		} else {
			// Don't let the user control where we place the directory.
			dirID := digest.FromString(newMount.Destination).Encoded()[:16]
			thisCacheRoot = filepath.Join(cacheParent, dirID)
			buildahLockFilesDir = filepath.Join(BuildahCacheLockfileDir, dirID)
		}
		idPair := idtools.IDPair{
			UID: uid,
			GID: gid,
		}
		// buildkit parity: change uid and gid if specified otheriwise keep `0`
		err = idtools.MkdirAllAndChownNew(thisCacheRoot, os.FileMode(mode), idPair)
		if err != nil {
			return newMount, "", nil, fmt.Errorf("unable to change uid,gid of cache directory: %w", err)
		}

		// create a subdirectory inside `cacheParent` just to store lockfiles
		buildahLockFilesDir = filepath.Join(cacheParent, buildahLockFilesDir)
		err = os.MkdirAll(buildahLockFilesDir, os.FileMode(0700))
		if err != nil {
			return newMount, "", nil, fmt.Errorf("unable to create build cache lockfiles directory: %w", err)
		}
	}

	// path should be /mountPoint/specified path
	evaluated, err := copier.Eval(thisCacheRoot, thisCacheRoot+string(filepath.Separator)+newMount.Source, copier.EvalOptions{})
	if err != nil {
		return newMount, "", nil, err
	}
	newMount.Source = evaluated

	var targetLock *lockfile.LockFile // = nil
	succeeded := false
	defer func() {
		if !succeeded && targetLock != nil {
			targetLock.Unlock()
		}
	}()
	switch sharing {
	case "locked":
		// lock parent cache
		lockfile, err := lockfile.GetLockFile(filepath.Join(buildahLockFilesDir, BuildahCacheLockfile))
		if err != nil {
			return newMount, "", nil, fmt.Errorf("unable to acquire lock when sharing mode is locked: %w", err)
		}
		// Will be unlocked after the RUN step is executed.
		lockfile.Lock()
		targetLock = lockfile
		defer func() {
			if !succeeded {
				targetLock.Unlock()
			}
		}()
	case "shared":
		// do nothing since default is `shared`
		break
	default:
		// error out for unknown values
		return newMount, "", nil, fmt.Errorf("unrecognized value %q for field `sharing`: %w", sharing, err)
	}

	// buildkit parity: default sharing should be shared
	// unless specified
	if !setShared {
		newMount.Options = append(newMount.Options, "shared")
	}

	// buildkit parity: cache must writable unless `ro` or `readonly` is configured explicitly
	if !setReadOnly {
		newMount.Options = append(newMount.Options, "rw")
	}

	newMount.Options = append(newMount.Options, "bind")

	opts, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, "", nil, err
	}
	newMount.Options = opts

	var intermediateMount string
	if newMount.Source != thisCacheRoot {
		rel, err := filepath.Rel(thisCacheRoot, newMount.Source)
		if err != nil {
			return newMount, "", nil, fmt.Errorf("computing pathname of cache subdirectory: %w", err)
		}
		if rel != "." && rel != "/" {
			mnt, err := internalVolumes.BindFromChroot(thisCacheRoot, rel, tmpDir)
			if err != nil {
				return newMount, "", nil, fmt.Errorf("sanitizing cache subdirectory %q: %w", newMount.Source, err)
			}
			logrus.Debugf("bind-mounted %q under %q to %q", rel, thisCacheRoot, mnt)
			intermediateMount = mnt
			newMount.Source = intermediateMount
		}
	}

	succeeded = true
	return newMount, intermediateMount, targetLock, nil
}

// ValidateVolumeMountHostDir validates the host path of buildah --volume
func ValidateVolumeMountHostDir(hostDir string) error {
	if !filepath.IsAbs(hostDir) {
		return fmt.Errorf("invalid host path, must be an absolute path %q", hostDir)
	}
	if _, err := os.Stat(hostDir); err != nil {
		return err
	}
	return nil
}

// RevertEscapedColon converts "\:" to ":"
func RevertEscapedColon(source string) string {
	return strings.ReplaceAll(source, "\\:", ":")
}

// SplitStringWithColonEscape splits string into slice by colon. Backslash-escaped colon (i.e. "\:") will not be regarded as separator
func SplitStringWithColonEscape(str string) []string {
	result := make([]string, 0, 3)
	sb := &strings.Builder{}
	for idx, r := range str {
		if r == ':' {
			// the colon is backslash-escaped
			if idx-1 > 0 && str[idx-1] == '\\' {
				sb.WriteRune(r)
			} else {
				// os.Stat will fail if path contains escaped colon
				result = append(result, RevertEscapedColon(sb.String()))
				sb.Reset()
			}
		} else {
			sb.WriteRune(r)
		}
	}
	if sb.Len() > 0 {
		result = append(result, RevertEscapedColon(sb.String()))
	}
	return result
}

func getVolumeMounts(volumes []string) (map[string]specs.Mount, error) {
	finalVolumeMounts := make(map[string]specs.Mount)

	for _, volume := range volumes {
		volumeMount, err := Volume(volume)
		if err != nil {
			return nil, err
		}
		if _, ok := finalVolumeMounts[volumeMount.Destination]; ok {
			return nil, fmt.Errorf("%v: %w", volumeMount.Destination, errDuplicateDest)
		}
		finalVolumeMounts[volumeMount.Destination] = volumeMount
	}
	return finalVolumeMounts, nil
}

// Volume parses the input of --volume
func Volume(volume string) (specs.Mount, error) {
	mount := specs.Mount{}
	arr := SplitStringWithColonEscape(volume)
	if len(arr) < 2 {
		return mount, fmt.Errorf("incorrect volume format %q, should be host-dir:ctr-dir[:option]", volume)
	}
	if err := ValidateVolumeMountHostDir(arr[0]); err != nil {
		return mount, err
	}
	if err := parse.ValidateVolumeCtrDir(arr[1]); err != nil {
		return mount, err
	}
	mountOptions := ""
	if len(arr) > 2 {
		mountOptions = arr[2]
		if _, err := parse.ValidateVolumeOpts(strings.Split(arr[2], ",")); err != nil {
			return mount, err
		}
	}
	mountOpts := strings.Split(mountOptions, ",")
	mount.Source = arr[0]
	mount.Destination = arr[1]
	mount.Type = "rbind"
	mount.Options = mountOpts
	return mount, nil
}

// GetVolumes gets the volumes from --volume and --mount flags.
//
// Returns a slice of Mounts to add to the runtime spec's list of mounts, the
// IDs of any images we mounted, a slice of bind-mounted paths, a slice of
// overlay directories and a slice of locks that we acquired, or an error.
//
// The caller is expected to, after the command which uses the mounts and
// volumes exits, clean up the overlay directories, unmount and remove the
// mountpoints for the bind-mounted paths, unmount any images we mounted, and
// release the locks we returned if any.
func GetVolumes(ctx *types.SystemContext, store storage.Store, mountLabel string, volumes []string, mounts []string, contextDir, workDir, tmpDir string) ([]specs.Mount, []string, []string, []string, []*lockfile.LockFile, error) {
	unifiedMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, err := getMounts(ctx, store, mountLabel, mounts, contextDir, workDir, tmpDir)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			for _, overlayMount := range overlayMounts {
				if err := overlay.RemoveTemp(overlayMount); err != nil {
					logrus.Debugf("unmounting overlay mount at %q: %v", overlayMount, err)
				}
			}
			for _, intermediateMount := range intermediateMounts {
				if err := mount.Unmount(intermediateMount); err != nil {
					logrus.Debugf("unmounting intermediate mount point %q: %v", intermediateMount, err)
				}
				if err := os.Remove(intermediateMount); err != nil {
					logrus.Debugf("removing should-be-empty directory %q: %v", intermediateMount, err)
				}
			}
			for _, image := range mountedImages {
				if _, err := store.UnmountImage(image, false); err != nil {
					logrus.Debugf("unmounting image %q: %v", image, err)
				}
			}
			UnlockLockArray(targetLocks)
		}
	}()
	volumeMounts, err := getVolumeMounts(volumes)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for dest, mount := range volumeMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", dest, errDuplicateDest)
		}
		unifiedMounts[dest] = mount
	}

	finalMounts := make([]specs.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		finalMounts = append(finalMounts, mount)
	}
	succeeded = true
	return finalMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, nil
}

// getMounts takes user-provided inputs from the --mount flag and returns a
// slice of OCI spec mounts, a slice of mounted image IDs, a slice of other
// mount locations, a slice of overlay mounts, and a slice of locks, or an
// error.
//
//     buildah run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
//     buildah run --mount type=cache,target=/var/cache ...
//     buildah run --mount type=tmpfs,target=/dev/shm ...
//
// The caller is expected to, after the command which uses the mounts exits,
// unmount the overlay filesystems (if we mounted any), unmount the other
// mounted filesystems and remove their mountpoints (if we provided any paths
// to mountpoints), unmount any mounted images (if we provided the IDs of any),
// and then unlock the locks we returned if any.
func getMounts(ctx *types.SystemContext, store storage.Store, mountLabel string, mounts []string, contextDir, workDir, tmpDir string) (map[string]specs.Mount, []string, []string, []string, []*lockfile.LockFile, error) {
	// If `type` is not set default to "bind"
	mountType := define.TypeBind
	finalMounts := make(map[string]specs.Mount, len(mounts))
	mountedImages := make([]string, 0, len(mounts))
	intermediateMounts := make([]string, 0, len(mounts))
	overlayMounts := make([]string, 0, len(mounts))
	targetLocks := make([]*lockfile.LockFile, 0)
	succeeded := false
	defer func() {
		if !succeeded {
			for _, overlayDir := range overlayMounts {
				if err := overlay.RemoveTemp(overlayDir); err != nil {
					logrus.Debugf("unmounting overlay mount at %q: %v", overlayDir, err)
				}
			}
			for _, intermediateMount := range intermediateMounts {
				if err := mount.Unmount(intermediateMount); err != nil {
					logrus.Debugf("unmounting intermediate mount point %q: %v", intermediateMount, err)
				}
				if err := os.Remove(intermediateMount); err != nil {
					logrus.Debugf("removing should-be-empty directory %q: %v", intermediateMount, err)
				}
			}
			for _, image := range mountedImages {
				if _, err := store.UnmountImage(image, false); err != nil {
					logrus.Debugf("unmounting image %q: %v", image, err)
				}
			}
			UnlockLockArray(targetLocks)
		}
	}()

	errInvalidSyntax := errors.New("incorrect mount format: should be --mount type=<bind|tmpfs>,[src=<host-dir>,]target=<ctr-dir>[,options]")

	// TODO(vrothberg): the manual parsing can be replaced with a regular expression
	//                  to allow a more robust parsing of the mount format and to give
	//                  precise errors regarding supported format versus supported options.
	for _, mount := range mounts {
		tokens := strings.Split(mount, ",")
		if len(tokens) < 2 {
			return nil, nil, nil, nil, nil, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
		}
		for _, field := range tokens {
			if strings.HasPrefix(field, "type=") {
				kv := strings.Split(field, "=")
				if len(kv) != 2 {
					return nil, nil, nil, nil, nil, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
				}
				mountType = kv[1]
			}
		}
		switch mountType {
		case define.TypeBind:
			mount, image, intermediateMount, overlayMount, err := GetBindMount(ctx, tokens, contextDir, store, mountLabel, nil, workDir, tmpDir)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if image != "" {
				mountedImages = append(mountedImages, image)
			}
			if intermediateMount != "" {
				intermediateMounts = append(intermediateMounts, intermediateMount)
			}
			if overlayMount != "" {
				overlayMounts = append(overlayMounts, overlayMount)
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case TypeCache:
			mount, intermediateMount, tl, err := GetCacheMount(tokens, nil, workDir, tmpDir)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if intermediateMount != "" {
				intermediateMounts = append(intermediateMounts, intermediateMount)
			}
			if tl != nil {
				targetLocks = append(targetLocks, tl)
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case TypeTmpfs:
			mount, err := GetTmpfsMount(tokens)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		default:
			return nil, nil, nil, nil, nil, fmt.Errorf("invalid filesystem type %q", mountType)
		}
	}

	succeeded = true
	return finalMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, nil
}

// GetTmpfsMount parses a single tmpfs mount entry from the --mount flag
func GetTmpfsMount(args []string) (specs.Mount, error) {
	newMount := specs.Mount{
		Type:   TypeTmpfs,
		Source: TypeTmpfs,
	}

	setDest := false

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "type":
			// This is already processed
			continue
		case "ro", "nosuid", "nodev", "noexec":
			newMount.Options = append(newMount.Options, kv[0])
		case "readonly":
			// Alias for "ro"
			newMount.Options = append(newMount.Options, "ro")
		case "tmpcopyup":
			//the path that is shadowed by the tmpfs mount is recursively copied up to the tmpfs itself.
			newMount.Options = append(newMount.Options, kv[0])
		case "tmpfs-mode":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("mode=%s", kv[1]))
		case "tmpfs-size":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("size=%s", kv[1]))
		case "src", "source":
			return newMount, errors.New("source is not supported with tmpfs mounts")
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = kv[1]
			setDest = true
		default:
			return newMount, fmt.Errorf("%v: %w", kv[0], errBadMntOption)
		}
	}

	if !setDest {
		return newMount, errBadVolDest
	}

	return newMount, nil
}

// UnlockLockArray is a helper for cleaning up after GetVolumes and the like.
func UnlockLockArray(locks []*lockfile.LockFile) {
	for _, lock := range locks {
		lock.Unlock()
	}
}
