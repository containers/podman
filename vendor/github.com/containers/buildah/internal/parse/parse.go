package parse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"errors"

	"github.com/containers/buildah/internal"
	internalUtil "github.com/containers/buildah/internal/util"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/lockfile"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
	// TypeCache is the type for mounting a common persistent cache from host
	TypeCache = "cache"
	// mount=type=cache must create a persistent directory on host so its available for all consecutive builds.
	// Lifecycle of following directory will be inherited from how host machine treats temporary directory
	BuildahCacheDir = "buildah-cache"
	// mount=type=cache allows users to lock a cache store while its being used by another build
	BuildahCacheLockfile = "buildah-cache-lockfile"
)

var (
	errBadMntOption  = errors.New("invalid mount option")
	errBadOptionArg  = errors.New("must provide an argument for option")
	errBadVolDest    = errors.New("must set volume destination")
	errBadVolSrc     = errors.New("must set volume source")
	errDuplicateDest = errors.New("duplicate mount destination")
)

// GetBindMount parses a single bind mount entry from the --mount flag.
// Returns specifiedMount and a string which contains name of image that we mounted otherwise its empty.
// Caller is expected to perform unmount of any mounted images
func GetBindMount(ctx *types.SystemContext, args []string, contextDir string, store storage.Store, imageMountLabel string, additionalMountPoints map[string]internal.StageMountDetails) (specs.Mount, string, error) {
	newMount := specs.Mount{
		Type: TypeBind,
	}

	mountReadability := false
	setDest := false
	bindNonRecursive := false
	fromImage := ""

	for _, val := range args {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "bind-nonrecursive":
			newMount.Options = append(newMount.Options, "bind")
			bindNonRecursive = true
		case "ro", "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options.
			// (Is this necessary?)
			newMount.Options = append(newMount.Options, kv[0])
			mountReadability = true
		case "rw", "readwrite":
			newMount.Options = append(newMount.Options, "rw")
			mountReadability = true
		case "readonly":
			// Alias for "ro"
			newMount.Options = append(newMount.Options, "ro")
			mountReadability = true
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z", "U":
			newMount.Options = append(newMount.Options, kv[0])
		case "from":
			if len(kv) == 1 {
				return newMount, "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			fromImage = kv[1]
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "src", "source":
			if len(kv) == 1 {
				return newMount, "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Source = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, "", fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, "", err
			}
			newMount.Destination = kv[1]
			setDest = true
		case "consistency":
			// Option for OS X only, has no meaning on other platforms
			// and can thus be safely ignored.
			// See also the handling of the equivalent "delegated" and "cached" in ValidateVolumeOpts
		default:
			return newMount, "", fmt.Errorf("%v: %w", kv[0], errBadMntOption)
		}
	}

	// default mount readability is always readonly
	if !mountReadability {
		newMount.Options = append(newMount.Options, "ro")
	}

	// Following variable ensures that we return imagename only if we did additional mount
	isImageMounted := false
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
			image, err := internalUtil.LookupImage(ctx, store, fromImage)
			if err != nil {
				return newMount, "", err
			}

			mountPoint, err = image.Mount(context.Background(), nil, imageMountLabel)
			if err != nil {
				return newMount, "", err
			}
			isImageMounted = true
		}
		contextDir = mountPoint
	}

	// buildkit parity: default bind option must be `rbind`
	// unless specified
	if !bindNonRecursive {
		newMount.Options = append(newMount.Options, "rbind")
	}

	if !setDest {
		return newMount, fromImage, errBadVolDest
	}

	// buildkit parity: support absolute path for sources from current build context
	if contextDir != "" {
		// path should be /contextDir/specified path
		newMount.Source = filepath.Join(contextDir, filepath.Clean(string(filepath.Separator)+newMount.Source))
	} else {
		// looks like its coming from `build run --mount=type=bind` allow using absolute path
		// error out if no source is set
		if newMount.Source == "" {
			return newMount, "", errBadVolSrc
		}
		if err := parse.ValidateVolumeHostDir(newMount.Source); err != nil {
			return newMount, "", err
		}
	}

	opts, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, fromImage, err
	}
	newMount.Options = opts

	if !isImageMounted {
		// we don't want any cleanups if image was not mounted explicitly
		// so dont return anything
		fromImage = ""
	}

	return newMount, fromImage, nil
}

// GetCacheMount parses a single cache mount entry from the --mount flag.
func GetCacheMount(args []string, store storage.Store, imageMountLabel string, additionalMountPoints map[string]internal.StageMountDetails) (specs.Mount, []string, error) {
	var err error
	var mode uint64
	lockedTargets := make([]string, 0)
	var (
		setDest     bool
		setShared   bool
		setReadOnly bool
	)
	fromStage := ""
	newMount := specs.Mount{
		Type: TypeBind,
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
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z", "U":
			newMount.Options = append(newMount.Options, kv[0])
			setShared = true
		case "sharing":
			sharing = kv[1]
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "id":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			id = kv[1]
		case "from":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			fromStage = kv[1]
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			if err := parse.ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, lockedTargets, err
			}
			newMount.Destination = kv[1]
			setDest = true
		case "src", "source":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			newMount.Source = kv[1]
		case "mode":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			mode, err = strconv.ParseUint(kv[1], 8, 32)
			if err != nil {
				return newMount, lockedTargets, fmt.Errorf("unable to parse cache mode: %w", err)
			}
		case "uid":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			uid, err = strconv.Atoi(kv[1])
			if err != nil {
				return newMount, lockedTargets, fmt.Errorf("unable to parse cache uid: %w", err)
			}
		case "gid":
			if len(kv) == 1 {
				return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadOptionArg)
			}
			gid, err = strconv.Atoi(kv[1])
			if err != nil {
				return newMount, lockedTargets, fmt.Errorf("unable to parse cache gid: %w", err)
			}
		default:
			return newMount, lockedTargets, fmt.Errorf("%v: %w", kv[0], errBadMntOption)
		}
	}

	if !setDest {
		return newMount, lockedTargets, errBadVolDest
	}

	if fromStage != "" {
		// do not create cache on host
		// instead use read-only mounted stage as cache
		mountPoint := ""
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[fromStage]; ok {
				if val.IsStage {
					mountPoint = val.MountPoint
				}
			}
		}
		// Cache does not supports using image so if not stage found
		// return with error
		if mountPoint == "" {
			return newMount, lockedTargets, fmt.Errorf("no stage found with name %s", fromStage)
		}
		// path should be /contextDir/specified path
		newMount.Source = filepath.Join(mountPoint, filepath.Clean(string(filepath.Separator)+newMount.Source))
	} else {
		// we need to create cache on host if no image is being used

		// since type is cache and cache can be reused by consecutive builds
		// create a common cache directory, which persists on hosts within temp lifecycle
		// add subdirectory if specified

		// cache parent directory
		cacheParent := filepath.Join(internalUtil.GetTempDir(), BuildahCacheDir)
		// create cache on host if not present
		err = os.MkdirAll(cacheParent, os.FileMode(0755))
		if err != nil {
			return newMount, lockedTargets, fmt.Errorf("unable to create build cache directory: %w", err)
		}

		if id != "" {
			newMount.Source = filepath.Join(cacheParent, filepath.Clean(id))
		} else {
			newMount.Source = filepath.Join(cacheParent, filepath.Clean(newMount.Destination))
		}
		idPair := idtools.IDPair{
			UID: uid,
			GID: gid,
		}
		//buildkit parity: change uid and gid if specified otheriwise keep `0`
		err = idtools.MkdirAllAndChownNew(newMount.Source, os.FileMode(mode), idPair)
		if err != nil {
			return newMount, lockedTargets, fmt.Errorf("unable to change uid,gid of cache directory: %w", err)
		}
	}

	switch sharing {
	case "locked":
		// lock parent cache
		lockfile, err := lockfile.GetLockfile(filepath.Join(newMount.Source, BuildahCacheLockfile))
		if err != nil {
			return newMount, lockedTargets, fmt.Errorf("unable to acquire lock when sharing mode is locked: %w", err)
		}
		// Will be unlocked after the RUN step is executed.
		lockfile.Lock()
		lockedTargets = append(lockedTargets, filepath.Join(newMount.Source, BuildahCacheLockfile))
	case "shared":
		// do nothing since default is `shared`
		break
	default:
		// error out for unknown values
		return newMount, lockedTargets, fmt.Errorf("unrecognized value %q for field `sharing`: %w", sharing, err)
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
		return newMount, lockedTargets, err
	}
	newMount.Options = opts

	return newMount, lockedTargets, nil
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

// GetVolumes gets the volumes from --volume and --mount
func GetVolumes(ctx *types.SystemContext, store storage.Store, volumes []string, mounts []string, contextDir string) ([]specs.Mount, []string, []string, error) {
	unifiedMounts, mountedImages, lockedTargets, err := getMounts(ctx, store, mounts, contextDir)
	if err != nil {
		return nil, mountedImages, lockedTargets, err
	}
	volumeMounts, err := getVolumeMounts(volumes)
	if err != nil {
		return nil, mountedImages, lockedTargets, err
	}
	for dest, mount := range volumeMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, mountedImages, lockedTargets, fmt.Errorf("%v: %w", dest, errDuplicateDest)
		}
		unifiedMounts[dest] = mount
	}

	finalMounts := make([]specs.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		finalMounts = append(finalMounts, mount)
	}
	return finalMounts, mountedImages, lockedTargets, nil
}

// getMounts takes user-provided input from the --mount flag and creates OCI
// spec mounts.
// buildah run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// buildah run --mount type=tmpfs,target=/dev/shm ...
func getMounts(ctx *types.SystemContext, store storage.Store, mounts []string, contextDir string) (map[string]specs.Mount, []string, []string, error) {
	finalMounts := make(map[string]specs.Mount)
	mountedImages := make([]string, 0)
	lockedTargets := make([]string, 0)

	errInvalidSyntax := errors.New("incorrect mount format: should be --mount type=<bind|tmpfs>,[src=<host-dir>,]target=<ctr-dir>[,options]")

	// TODO(vrothberg): the manual parsing can be replaced with a regular expression
	//                  to allow a more robust parsing of the mount format and to give
	//                  precise errors regarding supported format versus supported options.
	for _, mount := range mounts {
		arr := strings.SplitN(mount, ",", 2)
		if len(arr) < 2 {
			return nil, mountedImages, lockedTargets, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
		}
		kv := strings.Split(arr[0], "=")
		// TODO: type is not explicitly required in Docker.
		// If not specified, it defaults to "volume".
		if len(kv) != 2 || kv[0] != "type" {
			return nil, mountedImages, lockedTargets, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
		}

		tokens := strings.Split(arr[1], ",")
		switch kv[1] {
		case TypeBind:
			mount, image, err := GetBindMount(ctx, tokens, contextDir, store, "", nil)
			if err != nil {
				return nil, mountedImages, lockedTargets, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, mountedImages, lockedTargets, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
			mountedImages = append(mountedImages, image)
		case TypeCache:
			mount, lockedPaths, err := GetCacheMount(tokens, store, "", nil)
			lockedTargets = lockedPaths
			if err != nil {
				return nil, mountedImages, lockedTargets, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, mountedImages, lockedTargets, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case TypeTmpfs:
			mount, err := GetTmpfsMount(tokens)
			if err != nil {
				return nil, mountedImages, lockedTargets, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, mountedImages, lockedTargets, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		default:
			return nil, mountedImages, lockedTargets, fmt.Errorf("invalid filesystem type %q", kv[1])
		}
	}

	return finalMounts, mountedImages, lockedTargets, nil
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
