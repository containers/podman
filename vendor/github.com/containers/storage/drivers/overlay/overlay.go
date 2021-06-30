// +build linux

package overlay

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	graphdriver "github.com/containers/storage/drivers"
	"github.com/containers/storage/drivers/overlayutils"
	"github.com/containers/storage/drivers/quota"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/directory"
	"github.com/containers/storage/pkg/fsutils"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/locker"
	"github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/parsers"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	units "github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vbatts/tar-split/tar/storage"
	"golang.org/x/sys/unix"
)

var (
	// untar defines the untar method
	untar = chrootarchive.UntarUncompressed
)

const (
	defaultPerms     = os.FileMode(0555)
	selinuxLabelTest = "system_u:object_r:container_file_t:s0"
)

// This backend uses the overlay union filesystem for containers
// with diff directories for each layer.

// This version of the overlay driver requires at least kernel
// 4.0.0 in order to support mounting multiple diff directories.

// Each container/image has at least a "diff" directory and "link" file.
// If there is also a "lower" file when there are diff layers
// below as well as "merged" and "work" directories. The "diff" directory
// has the upper layer of the overlay and is used to capture any
// changes to the layer. The "lower" file contains all the lower layer
// mounts separated by ":" and ordered from uppermost to lowermost
// layers. The overlay itself is mounted in the "merged" directory,
// and the "work" dir is needed for overlay to work.

// The "link" file for each layer contains a unique string for the layer.
// Under the "l" directory at the root there will be a symbolic link
// with that unique string pointing the "diff" directory for the layer.
// The symbolic links are used to reference lower layers in the "lower"
// file and on mount. The links are used to shorten the total length
// of a layer reference without requiring changes to the layer identifier
// or root directory. Mounts are always done relative to root and
// referencing the symbolic links in order to ensure the number of
// lower directories can fit in a single page for making the mount
// syscall. A hard upper limit of 128 lower layers is enforced to ensure
// that mounts do not fail due to length.

const (
	linkDir   = "l"
	lowerFile = "lower"
	maxDepth  = 128

	// idLength represents the number of random characters
	// which can be used to create the unique link identifier
	// for every layer. If this value is too long then the
	// page size limit for the mount command may be exceeded.
	// The idLength should be selected such that following equation
	// is true (512 is a buffer for label metadata).
	// ((idLength + len(linkDir) + 1) * maxDepth) <= (pageSize - 512)
	idLength = 26
)

type overlayOptions struct {
	imageStores       []string
	layerStores       []additionalLayerStore
	quota             quota.Quota
	mountProgram      string
	skipMountHome     bool
	mountOptions      string
	ignoreChownErrors bool
	forceMask         *os.FileMode
}

// Driver contains information about the home directory and the list of active mounts that are created using this driver.
type Driver struct {
	name             string
	home             string
	runhome          string
	uidMaps          []idtools.IDMap
	gidMaps          []idtools.IDMap
	ctr              *graphdriver.RefCounter
	quotaCtl         *quota.Control
	options          overlayOptions
	naiveDiff        graphdriver.DiffDriver
	supportsDType    bool
	supportsVolatile *bool
	usingMetacopy    bool
	locker           *locker.Locker
}

type additionalLayerStore struct {

	// path is the directory where this store is available on the host.
	path string

	// withReference is true when the store contains image reference information (base64-encoded)
	// in its layer search path so the path to the diff will be
	//  <path>/base64(reference)/<layerdigest>/
	withReference bool
}

var (
	backingFs             = "<unknown>"
	projectQuotaSupported = false

	useNaiveDiffLock sync.Once
	useNaiveDiffOnly bool
)

func init() {
	graphdriver.Register("overlay", Init)
	graphdriver.Register("overlay2", Init)
}

func hasMetacopyOption(opts []string) bool {
	for _, s := range opts {
		if s == "metacopy=on" {
			return true
		}
	}
	return false
}

func hasVolatileOption(opts []string) bool {
	for _, s := range opts {
		if s == "volatile" {
			return true
		}
	}
	return false
}

func getMountProgramFlagFile(path string) string {
	return filepath.Join(path, ".has-mount-program")
}

func checkSupportVolatile(home, runhome string) (bool, error) {
	feature := fmt.Sprintf("volatile")
	volatileCacheResult, _, err := cachedFeatureCheck(runhome, feature)
	var usingVolatile bool
	if err == nil {
		if volatileCacheResult {
			logrus.Debugf("cached value indicated that volatile is being used")
		} else {
			logrus.Debugf("cached value indicated that volatile is not being used")
		}
		usingVolatile = volatileCacheResult
	} else {
		usingVolatile, err = doesVolatile(home)
		if err == nil {
			if usingVolatile {
				logrus.Debugf("overlay test mount indicated that volatile is being used")
			} else {
				logrus.Debugf("overlay test mount indicated that volatile is not being used")
			}
			if err = cachedFeatureRecord(runhome, feature, usingVolatile, ""); err != nil {
				return false, errors.Wrap(err, "error recording volatile-being-used status")
			}
		}
	}
	return usingVolatile, nil
}

func checkAndRecordOverlaySupport(fsMagic graphdriver.FsMagic, home, runhome string) (bool, error) {
	var supportsDType bool

	if os.Geteuid() != 0 {
		return false, nil
	}

	feature := "overlay"
	overlayCacheResult, overlayCacheText, err := cachedFeatureCheck(runhome, feature)
	if err == nil {
		if overlayCacheResult {
			logrus.Debugf("cached value indicated that overlay is supported")
		} else {
			logrus.Debugf("cached value indicated that overlay is not supported")
		}
		supportsDType = overlayCacheResult
		if !supportsDType {
			return false, errors.New(overlayCacheText)
		}
	} else {
		supportsDType, err = supportsOverlay(home, fsMagic, 0, 0)
		if err != nil {
			os.Remove(filepath.Join(home, linkDir))
			os.Remove(home)
			patherr, ok := err.(*os.PathError)
			if ok && patherr.Err == syscall.ENOSPC {
				return false, err
			}
			err = errors.Wrap(err, "kernel does not support overlay fs")
			if err2 := cachedFeatureRecord(runhome, feature, false, err.Error()); err2 != nil {
				return false, errors.Wrapf(err2, "error recording overlay not being supported (%v)", err)
			}
			return false, err
		}
		if err = cachedFeatureRecord(runhome, feature, supportsDType, ""); err != nil {
			return false, errors.Wrap(err, "error recording overlay support status")
		}
	}
	return supportsDType, nil
}

func (d *Driver) getSupportsVolatile() (bool, error) {
	if d.supportsVolatile != nil {
		return *d.supportsVolatile, nil
	}
	supportsVolatile, err := checkSupportVolatile(d.home, d.runhome)
	if err != nil {
		return false, err
	}
	d.supportsVolatile = &supportsVolatile
	return supportsVolatile, nil
}

// Init returns the a native diff driver for overlay filesystem.
// If overlay filesystem is not supported on the host, a wrapped graphdriver.ErrNotSupported is returned as error.
// If an overlay filesystem is not supported over an existing filesystem then a wrapped graphdriver.ErrIncompatibleFS is returned.
func Init(home string, options graphdriver.Options) (graphdriver.Driver, error) {
	opts, err := parseOptions(options.DriverOptions)
	if err != nil {
		return nil, err
	}

	fsMagic, err := graphdriver.GetFSMagic(home)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	if opts.mountProgram != "" {
		f, err := os.Create(getMountProgramFlagFile(home))
		if err == nil {
			f.Close()
		}
	} else {
		// check if they are running over btrfs, aufs, zfs, overlay, or ecryptfs
		if opts.forceMask != nil {
			return nil, errors.New("'force_mask' is supported only with 'mount_program'")
		}
		switch fsMagic {
		case graphdriver.FsMagicAufs, graphdriver.FsMagicZfs, graphdriver.FsMagicOverlay, graphdriver.FsMagicEcryptfs:
			return nil, errors.Wrapf(graphdriver.ErrIncompatibleFS, "'overlay' is not supported over %s, a mount_program is required", backingFs)
		}
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(options.UIDMaps, options.GIDMaps)
	if err != nil {
		return nil, err
	}

	// Create the driver home dir
	if err := idtools.MkdirAllAs(path.Join(home, linkDir), 0700, rootUID, rootGID); err != nil {
		return nil, err
	}
	runhome := filepath.Join(options.RunRoot, filepath.Base(home))
	if err := idtools.MkdirAllAs(runhome, 0700, rootUID, rootGID); err != nil {
		return nil, err
	}

	var usingMetacopy bool
	var supportsDType bool
	var supportsVolatile *bool
	if opts.mountProgram != "" {
		supportsDType = true
		t := true
		supportsVolatile = &t
	} else {
		supportsDType, err = checkAndRecordOverlaySupport(fsMagic, home, runhome)
		if err != nil {
			return nil, err
		}
		feature := fmt.Sprintf("metacopy(%s)", opts.mountOptions)
		metacopyCacheResult, _, err := cachedFeatureCheck(runhome, feature)
		if err == nil {
			if metacopyCacheResult {
				logrus.Debugf("cached value indicated that metacopy is being used")
			} else {
				logrus.Debugf("cached value indicated that metacopy is not being used")
			}
			usingMetacopy = metacopyCacheResult
		} else {
			usingMetacopy, err = doesMetacopy(home, opts.mountOptions)
			if err == nil {
				if usingMetacopy {
					logrus.Debugf("overlay test mount indicated that metacopy is being used")
				} else {
					logrus.Debugf("overlay test mount indicated that metacopy is not being used")
				}
				if err = cachedFeatureRecord(runhome, feature, usingMetacopy, ""); err != nil {
					return nil, errors.Wrap(err, "error recording metacopy-being-used status")
				}
			} else {
				logrus.Infof("overlay test mount did not indicate whether or not metacopy is being used: %v", err)
				return nil, err
			}
		}
	}

	if !opts.skipMountHome {
		if err := mount.MakePrivate(home); err != nil {
			return nil, err
		}
	}

	fileSystemType := graphdriver.FsMagicOverlay
	if opts.mountProgram != "" {
		fileSystemType = graphdriver.FsMagicFUSE
	}

	d := &Driver{
		name:             "overlay",
		home:             home,
		runhome:          runhome,
		uidMaps:          options.UIDMaps,
		gidMaps:          options.GIDMaps,
		ctr:              graphdriver.NewRefCounter(graphdriver.NewFsChecker(fileSystemType)),
		supportsDType:    supportsDType,
		usingMetacopy:    usingMetacopy,
		supportsVolatile: supportsVolatile,
		locker:           locker.New(),
		options:          *opts,
	}

	d.naiveDiff = graphdriver.NewNaiveDiffDriver(d, graphdriver.NewNaiveLayerIDMapUpdater(d))
	if backingFs == "xfs" {
		// Try to enable project quota support over xfs.
		if d.quotaCtl, err = quota.NewControl(home); err == nil {
			projectQuotaSupported = true
		} else if opts.quota.Size > 0 {
			return nil, fmt.Errorf("Storage option overlay.size not supported. Filesystem does not support Project Quota: %v", err)
		}
	} else if opts.quota.Size > 0 {
		// if xfs is not the backing fs then error out if the storage-opt overlay.size is used.
		return nil, fmt.Errorf("Storage option overlay.size only supported for backingFS XFS. Found %v", backingFs)
	}

	logrus.Debugf("backingFs=%s, projectQuotaSupported=%v, useNativeDiff=%v, usingMetacopy=%v", backingFs, projectQuotaSupported, !d.useNaiveDiff(), d.usingMetacopy)

	return d, nil
}

func parseOptions(options []string) (*overlayOptions, error) {
	o := &overlayOptions{}
	for _, option := range options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		trimkey := strings.ToLower(key)
		trimkey = strings.TrimPrefix(trimkey, "overlay.")
		trimkey = strings.TrimPrefix(trimkey, "overlay2.")
		trimkey = strings.TrimPrefix(trimkey, ".")
		switch trimkey {
		case "override_kernel_check":
			logrus.Debugf("overlay: override_kernel_check option was specified, but is no longer necessary")
		case "mountopt":
			o.mountOptions = val
		case "size":
			logrus.Debugf("overlay: size=%s", val)
			size, err := units.RAMInBytes(val)
			if err != nil {
				return nil, err
			}
			o.quota.Size = uint64(size)
		case "imagestore", "additionalimagestore":
			logrus.Debugf("overlay: imagestore=%s", val)
			// Additional read only image stores to use for lower paths
			if val == "" {
				continue
			}
			for _, store := range strings.Split(val, ",") {
				store = filepath.Clean(store)
				if !filepath.IsAbs(store) {
					return nil, fmt.Errorf("overlay: image path %q is not absolute.  Can not be relative", store)
				}
				st, err := os.Stat(store)
				if err != nil {
					return nil, fmt.Errorf("overlay: can't stat imageStore dir %s: %v", store, err)
				}
				if !st.IsDir() {
					return nil, fmt.Errorf("overlay: image path %q must be a directory", store)
				}
				o.imageStores = append(o.imageStores, store)
			}
		case "additionallayerstore":
			logrus.Debugf("overlay: additionallayerstore=%s", val)
			// Additional read only layer stores to use for lower paths
			if val == "" {
				continue
			}
			for _, lstore := range strings.Split(val, ",") {
				elems := strings.Split(lstore, ":")
				lstore = filepath.Clean(elems[0])
				if !filepath.IsAbs(lstore) {
					return nil, fmt.Errorf("overlay: additionallayerstore path %q is not absolute.  Can not be relative", lstore)
				}
				st, err := os.Stat(lstore)
				if err != nil {
					return nil, errors.Wrap(err, "overlay: can't stat additionallayerstore dir")
				}
				if !st.IsDir() {
					return nil, fmt.Errorf("overlay: additionallayerstore path %q must be a directory", lstore)
				}
				var withReference bool
				for _, e := range elems[1:] {
					switch e {
					case "ref":
						if withReference {
							return nil, fmt.Errorf("overlay: additionallayerstore config of %q contains %q option twice", lstore, e)
						}
						withReference = true
					default:
						return nil, fmt.Errorf("overlay: additionallayerstore config %q contains unknown option %q", lstore, e)
					}
				}
				o.layerStores = append(o.layerStores, additionalLayerStore{
					path:          lstore,
					withReference: withReference,
				})
			}
		case "mount_program":
			logrus.Debugf("overlay: mount_program=%s", val)
			if val != "" {
				_, err := os.Stat(val)
				if err != nil {
					return nil, errors.Wrapf(err, "overlay: can't stat program %q", val)
				}
			}
			o.mountProgram = val
		case "skip_mount_home":
			logrus.Debugf("overlay: skip_mount_home=%s", val)
			o.skipMountHome, err = strconv.ParseBool(val)
		case "ignore_chown_errors":
			logrus.Debugf("overlay: ignore_chown_errors=%s", val)
			o.ignoreChownErrors, err = strconv.ParseBool(val)
			if err != nil {
				return nil, err
			}
		case "force_mask":
			logrus.Debugf("overlay: force_mask=%s", val)
			var mask int64
			switch val {
			case "shared":
				mask = 0755
			case "private":
				mask = 0700
			default:
				mask, err = strconv.ParseInt(val, 8, 32)
				if err != nil {
					return nil, err
				}
			}
			m := os.FileMode(mask)
			o.forceMask = &m
		default:
			return nil, fmt.Errorf("overlay: Unknown option %s", key)
		}
	}
	return o, nil
}

func cachedFeatureSet(feature string, set bool) string {
	if set {
		return fmt.Sprintf("%s-true", feature)
	}
	return fmt.Sprintf("%s-false", feature)
}

func cachedFeatureCheck(runhome, feature string) (supported bool, text string, err error) {
	content, err := ioutil.ReadFile(filepath.Join(runhome, cachedFeatureSet(feature, true)))
	if err == nil {
		return true, string(content), nil
	}
	content, err = ioutil.ReadFile(filepath.Join(runhome, cachedFeatureSet(feature, false)))
	if err == nil {
		return false, string(content), nil
	}
	return false, "", err
}

func cachedFeatureRecord(runhome, feature string, supported bool, text string) (err error) {
	f, err := os.Create(filepath.Join(runhome, cachedFeatureSet(feature, supported)))
	if f != nil {
		if text != "" {
			fmt.Fprintf(f, "%s", text)
		}
		f.Close()
	}
	return err
}

func SupportsNativeOverlay(graphroot, rundir string) (bool, error) {
	if os.Geteuid() != 0 || graphroot == "" || rundir == "" {
		return false, nil
	}

	home := filepath.Join(graphroot, "overlay")
	runhome := filepath.Join(rundir, "overlay")

	if _, err := os.Stat(getMountProgramFlagFile(home)); err == nil {
		logrus.Debugf("overlay storage already configured with a mount-program")
		return false, nil
	}

	for _, dir := range []string{home, runhome} {
		if _, err := os.Stat(dir); err != nil {
			_ = idtools.MkdirAllAs(dir, 0700, 0, 0)
		}
	}

	fsMagic, err := graphdriver.GetFSMagic(home)
	if err != nil {
		return false, err
	}

	supportsDType, _ := checkAndRecordOverlaySupport(fsMagic, home, runhome)
	return supportsDType, nil
}

func supportsOverlay(home string, homeMagic graphdriver.FsMagic, rootUID, rootGID int) (supportsDType bool, err error) {
	// We can try to modprobe overlay first

	exec.Command("modprobe", "overlay").Run()

	logLevel := logrus.ErrorLevel
	if unshare.IsRootless() {
		logLevel = logrus.DebugLevel
	}

	layerDir, err := ioutil.TempDir(home, "compat")
	if err != nil {
		patherr, ok := err.(*os.PathError)
		if ok && patherr.Err == syscall.ENOSPC {
			return false, err
		}
	}
	if err == nil {
		// Check if reading the directory's contents populates the d_type field, which is required
		// for proper operation of the overlay filesystem.
		supportsDType, err = fsutils.SupportsDType(layerDir)
		if err != nil {
			return false, err
		}
		if !supportsDType {
			return false, overlayutils.ErrDTypeNotSupported("overlay", backingFs)
		}

		// Try a test mount in the specific location we're looking at using.
		mergedDir := filepath.Join(layerDir, "merged")
		lower1Dir := filepath.Join(layerDir, "lower1")
		lower2Dir := filepath.Join(layerDir, "lower2")
		upperDir := filepath.Join(layerDir, "upper")
		workDir := filepath.Join(layerDir, "work")
		defer func() {
			// Permitted to fail, since the various subdirectories
			// can be empty or not even there, and the home might
			// legitimately be not empty
			_ = unix.Unmount(mergedDir, unix.MNT_DETACH)
			_ = os.RemoveAll(layerDir)
			_ = os.Remove(home)
		}()
		_ = idtools.MkdirAs(mergedDir, 0700, rootUID, rootGID)
		_ = idtools.MkdirAs(lower1Dir, 0700, rootUID, rootGID)
		_ = idtools.MkdirAs(lower2Dir, 0700, rootUID, rootGID)
		_ = idtools.MkdirAs(upperDir, 0700, rootUID, rootGID)
		_ = idtools.MkdirAs(workDir, 0700, rootUID, rootGID)
		flags := fmt.Sprintf("lowerdir=%s:%s,upperdir=%s,workdir=%s", lower1Dir, lower2Dir, upperDir, workDir)
		if selinux.GetEnabled() &&
			selinux.SecurityCheckContext(selinuxLabelTest) == nil {
			// Linux 5.11 introduced unprivileged overlay mounts but it has an issue
			// when used together with selinux labels.
			// Check that overlay supports selinux labels as well.
			flags = label.FormatMountLabel(flags, selinuxLabelTest)
		}
		if unshare.IsRootless() {
			flags = fmt.Sprintf("%s,userxattr", flags)
		}

		if len(flags) < unix.Getpagesize() {
			err := unix.Mount("overlay", mergedDir, "overlay", 0, flags)
			if err == nil {
				logrus.Debugf("overlay test mount with multiple lowers succeeded")
				return supportsDType, nil
			}
			logrus.Debugf("overlay test mount with multiple lowers failed %v", err)
		}
		flags = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower1Dir, upperDir, workDir)
		if selinux.GetEnabled() {
			flags = label.FormatMountLabel(flags, selinuxLabelTest)
		}
		if len(flags) < unix.Getpagesize() {
			err := unix.Mount("overlay", mergedDir, "overlay", 0, flags)
			if err == nil {
				logrus.StandardLogger().Logf(logLevel, "overlay test mount with multiple lowers failed, but succeeded with a single lower")
				return supportsDType, errors.Wrap(graphdriver.ErrNotSupported, "kernel too old to provide multiple lowers feature for overlay")
			}
			logrus.Debugf("overlay test mount with a single lower failed %v", err)
		}
		logrus.StandardLogger().Logf(logLevel, "'overlay' is not supported over %s at %q", backingFs, home)
		return supportsDType, errors.Wrapf(graphdriver.ErrIncompatibleFS, "'overlay' is not supported over %s at %q", backingFs, home)
	}

	logrus.StandardLogger().Logf(logLevel, "'overlay' not found as a supported filesystem on this host. Please ensure kernel is new enough and has overlay support loaded.")
	return supportsDType, errors.Wrap(graphdriver.ErrNotSupported, "'overlay' not found as a supported filesystem on this host. Please ensure kernel is new enough and has overlay support loaded.")
}

func (d *Driver) useNaiveDiff() bool {
	useNaiveDiffLock.Do(func() {
		if d.options.mountProgram != "" {
			useNaiveDiffOnly = true
			return
		}
		feature := fmt.Sprintf("native-diff(%s)", d.options.mountOptions)
		nativeDiffCacheResult, nativeDiffCacheText, err := cachedFeatureCheck(d.runhome, feature)
		if err == nil {
			if nativeDiffCacheResult {
				logrus.Debugf("cached value indicated that native-diff is usable")
			} else {
				logrus.Debugf("cached value indicated that native-diff is not being used")
				logrus.Info(nativeDiffCacheText)
			}
			useNaiveDiffOnly = !nativeDiffCacheResult
			return
		}
		if err := doesSupportNativeDiff(d.home, d.options.mountOptions); err != nil {
			nativeDiffCacheText = fmt.Sprintf("Not using native diff for overlay, this may cause degraded performance for building images: %v", err)
			logrus.Info(nativeDiffCacheText)
			useNaiveDiffOnly = true
		}
		cachedFeatureRecord(d.runhome, feature, !useNaiveDiffOnly, nativeDiffCacheText)
	})
	return useNaiveDiffOnly
}

func (d *Driver) String() string {
	return d.name
}

// Status returns current driver information in a two dimensional string array.
// Output contains "Backing Filesystem" used in this implementation.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Backing Filesystem", backingFs},
		{"Supports d_type", strconv.FormatBool(d.supportsDType)},
		{"Native Overlay Diff", strconv.FormatBool(!d.useNaiveDiff())},
		{"Using metacopy", strconv.FormatBool(d.usingMetacopy)},
	}
}

// Metadata returns meta data about the overlay driver such as
// LowerDir, UpperDir, WorkDir and MergeDir used to store data.
func (d *Driver) Metadata(id string) (map[string]string, error) {
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	metadata := map[string]string{
		"WorkDir":   path.Join(dir, "work"),
		"MergedDir": path.Join(dir, "merged"),
		"UpperDir":  path.Join(dir, "diff"),
	}

	lowerDirs, err := d.getLowerDirs(id)
	if err != nil {
		return nil, err
	}
	if len(lowerDirs) > 0 {
		metadata["LowerDir"] = strings.Join(lowerDirs, ":")
	}

	return metadata, nil
}

// Cleanup any state created by overlay which should be cleaned when daemon
// is being shutdown. For now, we just have to unmount the bind mounted
// we had created.
func (d *Driver) Cleanup() error {
	_ = os.RemoveAll(d.getStagingDir())
	return mount.Unmount(d.home)
}

// LookupAdditionalLayer looks up additional layer store by the specified
// digest and ref and returns an object representing that layer.
// This API is experimental and can be changed without bumping the major version number.
// TODO: to remove the comment once it's no longer experimental.
func (d *Driver) LookupAdditionalLayer(dgst digest.Digest, ref string) (graphdriver.AdditionalLayer, error) {
	l, err := d.getAdditionalLayerPath(dgst, ref)
	if err != nil {
		return nil, err
	}
	// Tell the additional layer store that we use this layer.
	// This will increase reference counter on the store's side.
	// This will be decreased on Release() method.
	notifyUseAdditionalLayer(l)
	return &additionalLayer{
		path: l,
		d:    d,
	}, nil
}

// LookupAdditionalLayerByID looks up additional layer store by the specified
// ID and returns an object representing that layer.
// This API is experimental and can be changed without bumping the major version number.
// TODO: to remove the comment once it's no longer experimental.
func (d *Driver) LookupAdditionalLayerByID(id string) (graphdriver.AdditionalLayer, error) {
	l, err := d.getAdditionalLayerPathByID(id)
	if err != nil {
		return nil, err
	}
	// Tell the additional layer store that we use this layer.
	// This will increase reference counter on the store's side.
	// This will be decreased on Release() method.
	notifyUseAdditionalLayer(l)
	return &additionalLayer{
		path: l,
		d:    d,
	}, nil
}

// CreateFromTemplate creates a layer with the same contents and parent as another layer.
func (d *Driver) CreateFromTemplate(id, template string, templateIDMappings *idtools.IDMappings, parent string, parentIDMappings *idtools.IDMappings, opts *graphdriver.CreateOpts, readWrite bool) error {
	if readWrite {
		return d.CreateReadWrite(id, template, opts)
	}
	return d.Create(id, template, opts)
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	if opts != nil && len(opts.StorageOpt) != 0 && !projectQuotaSupported {
		return fmt.Errorf("--storage-opt is supported only for overlay over xfs with 'pquota' mount option")
	}

	if opts == nil {
		opts = &graphdriver.CreateOpts{
			StorageOpt: map[string]string{},
		}
	}

	if _, ok := opts.StorageOpt["size"]; !ok {
		if opts.StorageOpt == nil {
			opts.StorageOpt = map[string]string{}
		}
		opts.StorageOpt["size"] = strconv.FormatUint(d.options.quota.Size, 10)
	}

	return d.create(id, parent, opts)
}

// Create is used to create the upper, lower, and merge directories required for overlay fs for a given id.
// The parent filesystem is used to configure these directories for the overlay.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) (retErr error) {
	if opts != nil && len(opts.StorageOpt) != 0 {
		if _, ok := opts.StorageOpt["size"]; ok {
			return fmt.Errorf("--storage-opt size is only supported for ReadWrite Layers")
		}
	}

	return d.create(id, parent, opts)
}

func (d *Driver) create(id, parent string, opts *graphdriver.CreateOpts) (retErr error) {
	dir := d.dir(id)

	uidMaps := d.uidMaps
	gidMaps := d.gidMaps

	if opts != nil && opts.IDMappings != nil {
		uidMaps = opts.IDMappings.UIDs()
		gidMaps = opts.IDMappings.GIDs()
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return err
	}
	// Make the link directory if it does not exist
	if err := idtools.MkdirAllAs(path.Join(d.home, linkDir), 0700, rootUID, rootGID); err != nil {
		return err
	}
	if err := idtools.MkdirAllAs(path.Dir(dir), 0700, rootUID, rootGID); err != nil {
		return err
	}
	if parent != "" {
		st, err := system.Stat(d.dir(parent))
		if err != nil {
			return err
		}
		rootUID = int(st.UID())
		rootGID = int(st.GID())
	}
	if err := idtools.MkdirAs(dir, 0700, rootUID, rootGID); err != nil {
		return err
	}

	defer func() {
		// Clean up on failure
		if retErr != nil {
			os.RemoveAll(dir)
		}
	}()

	if d.quotaCtl != nil {
		quota := quota.Quota{}
		if opts != nil && len(opts.StorageOpt) > 0 {
			driver := &Driver{}
			if err := d.parseStorageOpt(opts.StorageOpt, driver); err != nil {
				return err
			}
			if driver.options.quota.Size > 0 {
				quota.Size = driver.options.quota.Size
			}

		}
		// Set container disk quota limit
		// If it is set to 0, we will track the disk usage, but not enforce a limit
		if err := d.quotaCtl.SetQuota(dir, quota); err != nil {
			return err
		}
	}

	perms := defaultPerms
	if d.options.forceMask != nil {
		perms = *d.options.forceMask
	}
	if parent != "" {
		st, err := system.Stat(filepath.Join(d.dir(parent), "diff"))
		if err != nil {
			return err
		}
		perms = os.FileMode(st.Mode())
	}

	if err := idtools.MkdirAs(path.Join(dir, "diff"), perms, rootUID, rootGID); err != nil {
		return err
	}

	lid := generateID(idLength)
	if err := os.Symlink(path.Join("..", id, "diff"), path.Join(d.home, linkDir, lid)); err != nil {
		return err
	}

	// Write link id to link file
	if err := ioutil.WriteFile(path.Join(dir, "link"), []byte(lid), 0644); err != nil {
		return err
	}

	if err := idtools.MkdirAs(path.Join(dir, "work"), 0700, rootUID, rootGID); err != nil {
		return err
	}
	if err := idtools.MkdirAs(path.Join(dir, "merged"), 0700, rootUID, rootGID); err != nil {
		return err
	}

	// if no parent directory, create a dummy lower directory and skip writing a "lowers" file
	if parent == "" {
		return idtools.MkdirAs(path.Join(dir, "empty"), 0700, rootUID, rootGID)
	}

	lower, err := d.getLower(parent)
	if err != nil {
		return err
	}
	if lower != "" {
		if err := ioutil.WriteFile(path.Join(dir, lowerFile), []byte(lower), 0666); err != nil {
			return err
		}
	}

	return nil
}

// Parse overlay storage options
func (d *Driver) parseStorageOpt(storageOpt map[string]string, driver *Driver) error {
	// Read size to set the disk project quota per container
	for key, val := range storageOpt {
		key := strings.ToLower(key)
		switch key {
		case "size":
			size, err := units.RAMInBytes(val)
			if err != nil {
				return err
			}
			driver.options.quota.Size = uint64(size)
		default:
			return fmt.Errorf("Unknown option %s", key)
		}
	}

	return nil
}

func (d *Driver) getLower(parent string) (string, error) {
	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		return "", err
	}

	// Read Parent link fileA
	parentLink, err := ioutil.ReadFile(path.Join(parentDir, "link"))
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		logrus.Warnf("Can't read parent link %q because it does not exist. Going through storage to recreate the missing links.", path.Join(parentDir, "link"))
		if err := d.recreateSymlinks(); err != nil {
			return "", errors.Wrap(err, "error recreating the links")
		}
		parentLink, err = ioutil.ReadFile(path.Join(parentDir, "link"))
		if err != nil {
			return "", err
		}
	}
	lowers := []string{path.Join(linkDir, string(parentLink))}

	parentLower, err := ioutil.ReadFile(path.Join(parentDir, lowerFile))
	if err == nil {
		parentLowers := strings.Split(string(parentLower), ":")
		lowers = append(lowers, parentLowers...)
	}
	return strings.Join(lowers, ":"), nil
}

func (d *Driver) dir(id string) string {
	newpath := path.Join(d.home, id)
	if _, err := os.Stat(newpath); err != nil {
		for _, p := range d.AdditionalImageStores() {
			l := path.Join(p, d.name, id)
			_, err = os.Stat(l)
			if err == nil {
				return l
			}
		}
	}
	return newpath
}

func (d *Driver) getLowerDirs(id string) ([]string, error) {
	var lowersArray []string
	lowers, err := ioutil.ReadFile(path.Join(d.dir(id), lowerFile))
	if err == nil {
		for _, s := range strings.Split(string(lowers), ":") {
			lower := d.dir(s)
			lp, err := os.Readlink(lower)
			// if the link does not exist, we lost the symlinks during a sudden reboot.
			// Let's go ahead and recreate those symlinks.
			if err != nil {
				if os.IsNotExist(err) {
					logrus.Warnf("Can't read link %q because it does not exist. A storage corruption might have occurred, attempting to recreate the missing symlinks. It might be best wipe the storage to avoid further errors due to storage corruption.", lower)
					if err := d.recreateSymlinks(); err != nil {
						return nil, fmt.Errorf("error recreating the missing symlinks: %v", err)
					}
					// let's call Readlink on lower again now that we have recreated the missing symlinks
					lp, err = os.Readlink(lower)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
			lowersArray = append(lowersArray, path.Clean(d.dir(path.Join("link", lp))))
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return lowersArray, nil
}

func (d *Driver) optsAppendMappings(opts string, uidMaps, gidMaps []idtools.IDMap) string {
	if uidMaps == nil {
		uidMaps = d.uidMaps
	}
	if gidMaps == nil {
		gidMaps = d.gidMaps
	}
	if uidMaps != nil {
		var uids, gids bytes.Buffer
		if len(uidMaps) == 1 && uidMaps[0].Size == 1 {
			uids.WriteString(fmt.Sprintf("squash_to_uid=%d", uidMaps[0].HostID))
		} else {
			uids.WriteString("uidmapping=")
			for _, i := range uidMaps {
				if uids.Len() > 0 {
					uids.WriteString(":")
				}
				uids.WriteString(fmt.Sprintf("%d:%d:%d", i.ContainerID, i.HostID, i.Size))
			}
		}
		if len(gidMaps) == 1 && gidMaps[0].Size == 1 {
			gids.WriteString(fmt.Sprintf("squash_to_gid=%d", gidMaps[0].HostID))
		} else {
			gids.WriteString("gidmapping=")
			for _, i := range gidMaps {
				if gids.Len() > 0 {
					gids.WriteString(":")
				}
				gids.WriteString(fmt.Sprintf("%d:%d:%d", i.ContainerID, i.HostID, i.Size))
			}
		}
		return fmt.Sprintf("%s,%s,%s", opts, uids.String(), gids.String())
	}
	return opts
}

// Remove cleans the directories that are created for this id.
func (d *Driver) Remove(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)

	dir := d.dir(id)
	lid, err := ioutil.ReadFile(path.Join(dir, "link"))
	if err == nil {
		if err := os.RemoveAll(path.Join(d.home, linkDir, string(lid))); err != nil {
			logrus.Debugf("Failed to remove link: %v", err)
		}
	}

	d.releaseAdditionalLayerByID(id)

	if err := system.EnsureRemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// recreateSymlinks goes through the driver's home directory and checks if the diff directory
// under each layer has a symlink created for it under the linkDir. If the symlink does not
// exist, it creates them
func (d *Driver) recreateSymlinks() error {
	// List all the directories under the home directory
	dirs, err := ioutil.ReadDir(d.home)
	if err != nil {
		return fmt.Errorf("error reading driver home directory %q: %v", d.home, err)
	}
	linksDir := filepath.Join(d.home, "l")
	// This makes the link directory if it doesn't exist
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	if err := idtools.MkdirAllAs(path.Join(d.home, linkDir), 0700, rootUID, rootGID); err != nil {
		return err
	}
	// Keep looping as long as we take some corrective action in each iteration
	var errs *multierror.Error
	madeProgress := true
	for madeProgress {
		errs = nil
		madeProgress = false
		// Check that for each layer, there's a link in "l" with the name in
		// the layer's "link" file that points to the layer's "diff" directory.
		for _, dir := range dirs {
			// Skip over the linkDir and anything that is not a directory
			if dir.Name() == linkDir || !dir.Mode().IsDir() {
				continue
			}
			// Read the "link" file under each layer to get the name of the symlink
			data, err := ioutil.ReadFile(path.Join(d.dir(dir.Name()), "link"))
			if err != nil {
				errs = multierror.Append(errs, errors.Wrapf(err, "error reading name of symlink for %q", dir))
				continue
			}
			linkPath := path.Join(d.home, linkDir, strings.Trim(string(data), "\n"))
			// Check if the symlink exists, and if it doesn't, create it again with the
			// name we got from the "link" file
			_, err = os.Lstat(linkPath)
			if err != nil && os.IsNotExist(err) {
				if err := os.Symlink(path.Join("..", dir.Name(), "diff"), linkPath); err != nil {
					errs = multierror.Append(errs, err)
					continue
				}
				madeProgress = true
			} else if err != nil {
				errs = multierror.Append(errs, errors.Wrapf(err, "error trying to stat %q", linkPath))
				continue
			}
		}
		// Now check if we somehow lost a "link" file, by making sure
		// that each symlink we have corresponds to one.
		links, err := ioutil.ReadDir(linksDir)
		if err != nil {
			errs = multierror.Append(errs, errors.Wrapf(err, "error reading links directory %q", linksDir))
			continue
		}
		// Go through all of the symlinks in the "l" directory
		for _, link := range links {
			// Read the symlink's target, which should be "../$layer/diff"
			target, err := os.Readlink(filepath.Join(linksDir, link.Name()))
			if err != nil {
				errs = multierror.Append(errs, errors.Wrapf(err, "error reading target of link %q", link))
				continue
			}
			targetComponents := strings.Split(target, string(os.PathSeparator))
			if len(targetComponents) != 3 || targetComponents[0] != ".." || targetComponents[2] != "diff" {
				errs = multierror.Append(errs, errors.Errorf("link target of %q looks weird: %q", link, target))
				// force the link to be recreated on the next pass
				os.Remove(filepath.Join(linksDir, link.Name()))
				madeProgress = true
				continue
			}
			// Reconstruct the name of the target's link file and check that
			// it has the basename of our symlink in it.
			targetID := targetComponents[1]
			linkFile := filepath.Join(d.dir(targetID), "link")
			data, err := ioutil.ReadFile(linkFile)
			if err != nil || string(data) != link.Name() {
				if err := ioutil.WriteFile(linkFile, []byte(link.Name()), 0644); err != nil {
					errs = multierror.Append(errs, errors.Wrapf(err, "error correcting link for layer %q", targetID))
					continue
				}
				madeProgress = true
			}
		}
	}
	if errs != nil {
		return errs.ErrorOrNil()
	}
	return nil
}

// Get creates and mounts the required file system for the given id and returns the mount path.
func (d *Driver) Get(id string, options graphdriver.MountOpts) (_ string, retErr error) {
	return d.get(id, false, options)
}

func (d *Driver) get(id string, disableShifting bool, options graphdriver.MountOpts) (_ string, retErr error) {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return "", err
	}
	readWrite := true

	if !d.SupportsShifting() || options.DisableShifting {
		disableShifting = true
	}

	optsList := options.Options
	if len(optsList) == 0 {
		optsList = strings.Split(d.options.mountOptions, ",")
	} else {
		// If metacopy=on is present in d.options.mountOptions it must be present in the mount
		// options otherwise the kernel refuses to follow the metacopy xattr.
		if hasMetacopyOption(strings.Split(d.options.mountOptions, ",")) && !hasMetacopyOption(options.Options) {
			if d.usingMetacopy {
				optsList = append(optsList, "metacopy=on")
			} else {
				logLevel := logrus.WarnLevel
				if unshare.IsRootless() {
					logLevel = logrus.DebugLevel
				}
				logrus.StandardLogger().Logf(logLevel, "ignoring metacopy option from storage.conf, not supported with booted kernel")
			}
		}
	}
	for _, o := range optsList {
		if o == "ro" {
			readWrite = false
			break
		}
	}

	lowers, err := ioutil.ReadFile(path.Join(dir, lowerFile))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	splitLowers := strings.Split(string(lowers), ":")
	if len(splitLowers) > maxDepth {
		return "", errors.New("max depth exceeded")
	}

	// absLowers is the list of lowers as absolute paths, which works well with additional stores.
	absLowers := []string{}
	// relLowers is the list of lowers as paths relative to the driver's home directory.
	relLowers := []string{}

	// Check if $link/../diff{1-*} exist.  If they do, add them, in order, as the front of the lowers
	// lists that we're building.  "diff" itself is the upper, so it won't be in the lists.
	link, err := ioutil.ReadFile(path.Join(dir, "link"))
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		logrus.Warnf("Can't read parent link %q because it does not exist. Going through storage to recreate the missing links.", path.Join(dir, "link"))
		if err := d.recreateSymlinks(); err != nil {
			return "", errors.Wrap(err, "error recreating the links")
		}
		link, err = ioutil.ReadFile(path.Join(dir, "link"))
		if err != nil {
			return "", err
		}
	}
	diffN := 1
	perms := defaultPerms
	if d.options.forceMask != nil {
		perms = *d.options.forceMask
	}
	permsKnown := false
	st, err := os.Stat(filepath.Join(dir, nameWithSuffix("diff", diffN)))
	if err == nil {
		perms = os.FileMode(st.Mode())
		permsKnown = true
	}
	for err == nil {
		absLowers = append(absLowers, filepath.Join(dir, nameWithSuffix("diff", diffN)))
		relLowers = append(relLowers, dumbJoin(string(link), "..", nameWithSuffix("diff", diffN)))
		diffN++
		st, err = os.Stat(filepath.Join(dir, nameWithSuffix("diff", diffN)))
		if err == nil && !permsKnown {
			perms = os.FileMode(st.Mode())
			permsKnown = true
		}
	}

	// For each lower, resolve its path, and append it and any additional diffN
	// directories to the lowers list.
	for _, l := range splitLowers {
		if l == "" {
			continue
		}
		lower := ""
		newpath := path.Join(d.home, l)
		if st, err := os.Stat(newpath); err != nil {
			for _, p := range d.AdditionalImageStores() {
				lower = path.Join(p, d.name, l)
				if st2, err2 := os.Stat(lower); err2 == nil {
					if !permsKnown {
						perms = os.FileMode(st2.Mode())
						permsKnown = true
					}
					break
				}
				lower = ""
			}
			// if it is a "not found" error, that means the symlinks were lost in a sudden reboot
			// so call the recreateSymlinks function to go through all the layer dirs and recreate
			// the symlinks with the name from their respective "link" files
			if lower == "" && os.IsNotExist(err) {
				logrus.Warnf("Can't stat lower layer %q because it does not exist. Going through storage to recreate the missing symlinks.", newpath)
				if err := d.recreateSymlinks(); err != nil {
					return "", fmt.Errorf("error recreating the missing symlinks: %v", err)
				}
				lower = newpath
			} else if lower == "" {
				return "", fmt.Errorf("Can't stat lower layer %q: %v", newpath, err)
			}
		} else {
			if !permsKnown {
				perms = os.FileMode(st.Mode())
				permsKnown = true
			}
			lower = newpath
		}
		absLowers = append(absLowers, lower)
		relLowers = append(relLowers, l)
		diffN = 1
		_, err = os.Stat(dumbJoin(lower, "..", nameWithSuffix("diff", diffN)))
		for err == nil {
			absLowers = append(absLowers, dumbJoin(lower, "..", nameWithSuffix("diff", diffN)))
			relLowers = append(relLowers, dumbJoin(l, "..", nameWithSuffix("diff", diffN)))
			diffN++
			_, err = os.Stat(dumbJoin(lower, "..", nameWithSuffix("diff", diffN)))
		}
	}

	if len(absLowers) == 0 {
		absLowers = append(absLowers, path.Join(dir, "empty"))
		relLowers = append(relLowers, path.Join(id, "empty"))
	}
	// user namespace requires this to move a directory from lower to upper.
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return "", err
	}
	diffDir := path.Join(dir, "diff")
	if err := idtools.MkdirAllAs(diffDir, perms, rootUID, rootGID); err != nil {
		return "", err
	}

	mergedDir := path.Join(dir, "merged")
	// Create the driver merged dir
	if err := idtools.MkdirAs(mergedDir, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return "", err
	}
	if count := d.ctr.Increment(mergedDir); count > 1 {
		return mergedDir, nil
	}
	defer func() {
		if retErr != nil {
			if c := d.ctr.Decrement(mergedDir); c <= 0 {
				if mntErr := unix.Unmount(mergedDir, 0); mntErr != nil {
					logrus.Errorf("error unmounting %v: %v", mergedDir, mntErr)
				}
			}
		}
	}()

	workdir := path.Join(dir, "work")

	var opts string
	if readWrite {
		opts = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(absLowers, ":"), diffDir, workdir)
	} else {
		opts = fmt.Sprintf("lowerdir=%s:%s", diffDir, strings.Join(absLowers, ":"))
	}
	if len(optsList) > 0 {
		opts = fmt.Sprintf("%s,%s", strings.Join(optsList, ","), opts)
	}

	if d.options.mountProgram == "" && unshare.IsRootless() {
		opts = fmt.Sprintf("%s,userxattr", opts)
	}

	// If "volatile" is not supported by the file system, just ignore the request
	if options.Volatile && !hasVolatileOption(strings.Split(opts, ",")) {
		supported, err := d.getSupportsVolatile()
		if err != nil {
			return "", err
		}
		if supported {
			opts = fmt.Sprintf("%s,volatile", opts)
		}
	}

	mountData := label.FormatMountLabel(opts, options.MountLabel)
	mountFunc := unix.Mount
	mountTarget := mergedDir

	pageSize := unix.Getpagesize()

	// Use relative paths and mountFrom when the mount data has exceeded
	// the page size. The mount syscall fails if the mount data cannot
	// fit within a page and relative links make the mount data much
	// smaller at the expense of requiring a fork exec to chroot.
	if d.options.mountProgram != "" {
		mountFunc = func(source string, target string, mType string, flags uintptr, label string) error {
			if !disableShifting {
				label = d.optsAppendMappings(label, options.UidMaps, options.GidMaps)
			}

			mountProgram := exec.Command(d.options.mountProgram, "-o", label, target)
			mountProgram.Dir = d.home
			var b bytes.Buffer
			mountProgram.Stderr = &b
			err := mountProgram.Run()
			if err != nil {
				output := b.String()
				if output == "" {
					output = "<stderr empty>"
				}
				return errors.Wrapf(err, "using mount program %s: %s", d.options.mountProgram, output)
			}
			return nil
		}
	} else if len(mountData) > pageSize {
		workdir = path.Join(id, "work")
		//FIXME: We need to figure out to get this to work with additional stores
		if readWrite {
			diffDir := path.Join(id, "diff")
			opts = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(relLowers, ":"), diffDir, workdir)
		} else {
			opts = fmt.Sprintf("lowerdir=%s", strings.Join(absLowers, ":"))
		}
		mountData = label.FormatMountLabel(opts, options.MountLabel)
		if len(mountData) > pageSize {
			return "", fmt.Errorf("cannot mount layer, mount label %q too large %d > page size %d", options.MountLabel, len(mountData), pageSize)
		}
		mountFunc = func(source string, target string, mType string, flags uintptr, label string) error {
			return mountFrom(d.home, source, target, mType, flags, label)
		}
		mountTarget = path.Join(id, "merged")
	}

	// overlay has a check in place to prevent mounting the same file system twice
	// if volatile was already specified.
	err = os.RemoveAll(filepath.Join(workdir, "work/incompat/volatile"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	flags, data := mount.ParseOptions(mountData)
	logrus.Debugf("overlay: mount_data=%s", mountData)
	if err := mountFunc("overlay", mountTarget, "overlay", uintptr(flags), data); err != nil {
		return "", fmt.Errorf("error creating overlay mount to %s, mount_data=%q: %v", mountTarget, mountData, err)
	}

	return mergedDir, nil
}

// Put unmounts the mount path created for the give id.
func (d *Driver) Put(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	mountpoint := path.Join(dir, "merged")
	if count := d.ctr.Decrement(mountpoint); count > 0 {
		return nil
	}
	if _, err := ioutil.ReadFile(path.Join(dir, lowerFile)); err != nil && !os.IsNotExist(err) {
		return err
	}

	unmounted := false

	if d.options.mountProgram != "" {
		// Attempt to unmount the FUSE mount using either fusermount or fusermount3.
		// If they fail, fallback to unix.Unmount
		for _, v := range []string{"fusermount3", "fusermount"} {
			err := exec.Command(v, "-u", mountpoint).Run()
			if err != nil && errors.Cause(err) != exec.ErrNotFound {
				logrus.Debugf("Error unmounting %s with %s - %v", mountpoint, v, err)
			}
			if err == nil {
				unmounted = true
				break
			}
		}
		// If fusermount|fusermount3 failed to unmount the FUSE file system, make sure all
		// pending changes are propagated to the file system
		if !unmounted {
			fd, err := unix.Open(mountpoint, unix.O_DIRECTORY, 0)
			if err == nil {
				if err := unix.Syncfs(fd); err != nil {
					logrus.Debugf("Error Syncfs(%s) - %v", mountpoint, err)
				}
				unix.Close(fd)
			}
		}
	}

	if !unmounted {
		if err := unix.Unmount(mountpoint, unix.MNT_DETACH); err != nil && !os.IsNotExist(err) {
			logrus.Debugf("Failed to unmount %s overlay: %s - %v", id, mountpoint, err)
		}
	}

	if err := unix.Rmdir(mountpoint); err != nil && !os.IsNotExist(err) {
		logrus.Debugf("Failed to remove mountpoint %s overlay: %s - %v", id, mountpoint, err)
	}

	return nil
}

// Exists checks to see if the id is already mounted.
func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}

// isParent returns if the passed in parent is the direct parent of the passed in layer
func (d *Driver) isParent(id, parent string) bool {
	lowers, err := d.getLowerDirs(id)
	if err != nil {
		return false
	}
	if parent == "" && len(lowers) > 0 {
		return false
	}

	parentDir := d.dir(parent)
	var ld string
	if len(lowers) > 0 {
		ld = filepath.Dir(lowers[0])
	}
	if ld == "" && parent == "" {
		return true
	}
	return ld == parentDir
}

func (d *Driver) getWhiteoutFormat() archive.WhiteoutFormat {
	whiteoutFormat := archive.OverlayWhiteoutFormat
	if d.options.mountProgram != "" {
		// If we are using a mount program, we are most likely running
		// as an unprivileged user that cannot use mknod, so fallback to the
		// AUFS whiteout format.
		whiteoutFormat = archive.AUFSWhiteoutFormat
	}
	return whiteoutFormat
}

type fileGetNilCloser struct {
	storage.FileGetter
}

func (f fileGetNilCloser) Close() error {
	return nil
}

func (d *Driver) getStagingDir() string {
	return filepath.Join(d.home, "staging")
}

// DiffGetter returns a FileGetCloser that can read files from the directory that
// contains files for the layer differences. Used for direct access for tar-split.
func (d *Driver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	p, err := d.getDiffPath(id)
	if err != nil {
		return nil, err
	}
	return fileGetNilCloser{storage.NewPathFileGetter(p)}, nil
}

// CleanupStagingDirectory cleanups the staging directory.
func (d *Driver) CleanupStagingDirectory(stagingDirectory string) error {
	return os.RemoveAll(stagingDirectory)
}

// ApplyDiff applies the changes in the new layer using the specified function
func (d *Driver) ApplyDiffWithDiffer(id, parent string, options *graphdriver.ApplyDiffOpts, differ graphdriver.Differ) (output graphdriver.DriverWithDifferOutput, err error) {
	var idMappings *idtools.IDMappings
	if options != nil {
		idMappings = options.Mappings
	}
	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}

	applyDir := ""

	if id == "" {
		err := os.MkdirAll(d.getStagingDir(), 0700)
		if err != nil && !os.IsExist(err) {
			return graphdriver.DriverWithDifferOutput{}, err
		}
		applyDir, err = ioutil.TempDir(d.getStagingDir(), "")
		if err != nil {
			return graphdriver.DriverWithDifferOutput{}, err
		}

	} else {
		var err error
		applyDir, err = d.getDiffPath(id)
		if err != nil {
			return graphdriver.DriverWithDifferOutput{}, err
		}
	}

	logrus.Debugf("Applying differ in %s", applyDir)

	out, err := differ.ApplyDiff(applyDir, &archive.TarOptions{
		UIDMaps:           idMappings.UIDs(),
		GIDMaps:           idMappings.GIDs(),
		IgnoreChownErrors: d.options.ignoreChownErrors,
		WhiteoutFormat:    d.getWhiteoutFormat(),
		InUserNS:          userns.RunningInUserNS(),
	})
	out.Target = applyDir
	return out, err
}

// ApplyDiffFromStagingDirectory applies the changes using the specified staging directory.
func (d *Driver) ApplyDiffFromStagingDirectory(id, parent, stagingDirectory string, diffOutput *graphdriver.DriverWithDifferOutput, options *graphdriver.ApplyDiffOpts) error {
	if filepath.Dir(stagingDirectory) != d.getStagingDir() {
		return fmt.Errorf("%q is not a staging directory", stagingDirectory)
	}

	diff, err := d.getDiffPath(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(diff); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stagingDirectory, diff)
}

// DifferTarget gets the location where files are stored for the layer.
func (d *Driver) DifferTarget(id string) (string, error) {
	return d.getDiffPath(id)
}

// ApplyDiff applies the new layer into a root
func (d *Driver) ApplyDiff(id, parent string, options graphdriver.ApplyDiffOpts) (size int64, err error) {

	if !d.isParent(id, parent) {
		if d.options.ignoreChownErrors {
			options.IgnoreChownErrors = d.options.ignoreChownErrors
		}
		if d.options.forceMask != nil {
			options.ForceMask = d.options.forceMask
		}
		return d.naiveDiff.ApplyDiff(id, parent, options)
	}

	idMappings := options.Mappings
	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}

	applyDir, err := d.getDiffPath(id)
	if err != nil {
		return 0, err
	}

	logrus.Debugf("Applying tar in %s", applyDir)
	// Overlay doesn't need the parent id to apply the diff
	if err := untar(options.Diff, applyDir, &archive.TarOptions{
		UIDMaps:           idMappings.UIDs(),
		GIDMaps:           idMappings.GIDs(),
		IgnoreChownErrors: d.options.ignoreChownErrors,
		ForceMask:         d.options.forceMask,
		WhiteoutFormat:    d.getWhiteoutFormat(),
		InUserNS:          userns.RunningInUserNS(),
	}); err != nil {
		return 0, err
	}

	return directory.Size(applyDir)
}

func (d *Driver) getDiffPath(id string) (string, error) {
	dir := d.dir(id)
	return redirectDiffIfAdditionalLayer(path.Join(dir, "diff"))
}

func (d *Driver) getLowerDiffPaths(id string) ([]string, error) {
	layers, err := d.getLowerDirs(id)
	if err != nil {
		return nil, err
	}
	for i, l := range layers {
		layers[i], err = redirectDiffIfAdditionalLayer(l)
		if err != nil {
			return nil, err
		}
	}
	return layers, nil
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (size int64, err error) {
	if d.options.mountProgram == "" && (d.useNaiveDiff() || !d.isParent(id, parent)) {
		return d.naiveDiff.DiffSize(id, idMappings, parent, parentMappings, mountLabel)
	}

	p, err := d.getDiffPath(id)
	if err != nil {
		return 0, err
	}
	return directory.Size(p)
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (d *Driver) Diff(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (io.ReadCloser, error) {
	if d.useNaiveDiff() || !d.isParent(id, parent) {
		return d.naiveDiff.Diff(id, idMappings, parent, parentMappings, mountLabel)
	}

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}

	lowerDirs, err := d.getLowerDiffPaths(id)
	if err != nil {
		return nil, err
	}

	diffPath, err := d.getDiffPath(id)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Tar with options on %s", diffPath)
	return archive.TarWithOptions(diffPath, &archive.TarOptions{
		Compression:    archive.Uncompressed,
		UIDMaps:        idMappings.UIDs(),
		GIDMaps:        idMappings.GIDs(),
		WhiteoutFormat: d.getWhiteoutFormat(),
		WhiteoutData:   lowerDirs,
	})
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) ([]archive.Change, error) {
	if d.useNaiveDiff() || !d.isParent(id, parent) {
		return d.naiveDiff.Changes(id, idMappings, parent, parentMappings, mountLabel)
	}
	// Overlay doesn't have snapshots, so we need to get changes from all parent
	// layers.
	diffPath, err := d.getDiffPath(id)
	if err != nil {
		return nil, err
	}
	layers, err := d.getLowerDiffPaths(id)
	if err != nil {
		return nil, err
	}

	return archive.OverlayChanges(layers, diffPath)
}

// AdditionalImageStores returns additional image stores supported by the driver
func (d *Driver) AdditionalImageStores() []string {
	return d.options.imageStores
}

// UpdateLayerIDMap updates ID mappings in a from matching the ones specified
// by toContainer to those specified by toHost.
func (d *Driver) UpdateLayerIDMap(id string, toContainer, toHost *idtools.IDMappings, mountLabel string) error {
	var err error
	dir := d.dir(id)
	diffDir := filepath.Join(dir, "diff")

	rootUID, rootGID := 0, 0
	if toHost != nil {
		rootUID, rootGID, err = idtools.GetRootUIDGID(toHost.UIDs(), toHost.GIDs())
		if err != nil {
			return err
		}
	}

	// Mount the new layer and handle ownership changes and possible copy_ups in it.
	options := graphdriver.MountOpts{
		MountLabel: mountLabel,
		Options:    strings.Split(d.options.mountOptions, ","),
	}
	layerFs, err := d.get(id, true, options)
	if err != nil {
		return err
	}
	err = graphdriver.ChownPathByMaps(layerFs, toContainer, toHost)
	if err != nil {
		if err2 := d.Put(id); err2 != nil {
			logrus.Errorf("%v; error unmounting %v: %v", err, id, err2)
		}
		return err
	}
	if err = d.Put(id); err != nil {
		return err
	}

	// Rotate the diff directories.
	i := 0
	perms := defaultPerms
	st, err := os.Stat(nameWithSuffix(diffDir, i))
	if d.options.forceMask != nil {
		perms = *d.options.forceMask
	} else {
		if err == nil {
			perms = os.FileMode(st.Mode())
		}
	}
	for err == nil {
		i++
		_, err = os.Stat(nameWithSuffix(diffDir, i))
	}

	for i > 0 {
		err = os.Rename(nameWithSuffix(diffDir, i-1), nameWithSuffix(diffDir, i))
		if err != nil {
			return err
		}
		i--
	}

	// We need to re-create the work directory as it might keep a reference
	// to the old upper layer in the index.
	workDir := filepath.Join(dir, "work")
	if err := os.RemoveAll(workDir); err == nil {
		if err := idtools.MkdirAs(workDir, defaultPerms, rootUID, rootGID); err != nil {
			return err
		}
	}

	// Re-create the directory that we're going to use as the upper layer.
	if err := idtools.MkdirAs(diffDir, perms, rootUID, rootGID); err != nil {
		return err
	}
	return nil
}

// SupportsShifting tells whether the driver support shifting of the UIDs/GIDs in an userNS
func (d *Driver) SupportsShifting() bool {
	if os.Getenv("_TEST_FORCE_SUPPORT_SHIFTING") == "yes-please" {
		return true
	}
	return d.options.mountProgram != ""
}

// dumbJoin is more or less a dumber version of filepath.Join, but one which
// won't Clean() the path, allowing us to append ".." as a component and trust
// pathname resolution to do some non-obvious work.
func dumbJoin(names ...string) string {
	if len(names) == 0 {
		return string(os.PathSeparator)
	}
	return strings.Join(names, string(os.PathSeparator))
}

func nameWithSuffix(name string, number int) string {
	if number == 0 {
		return name
	}
	return fmt.Sprintf("%s%d", name, number)
}

func (d *Driver) getAdditionalLayerPath(dgst digest.Digest, ref string) (string, error) {
	refElem := base64.StdEncoding.EncodeToString([]byte(ref))
	for _, ls := range d.options.layerStores {
		ref := ""
		if ls.withReference {
			ref = refElem
		}
		target := path.Join(ls.path, ref, dgst.String())
		// Check if all necessary files exist
		for _, p := range []string{
			filepath.Join(target, "diff"),
			filepath.Join(target, "info"),
			filepath.Join(target, "blob"),
		} {
			if _, err := os.Stat(p); err != nil {
				return "", errors.Wrapf(graphdriver.ErrLayerUnknown,
					"failed to stat additional layer %q: %v", p, err)
			}
		}
		return target, nil
	}

	return "", errors.Wrapf(graphdriver.ErrLayerUnknown,
		"additional layer (%q, %q) not found", dgst, ref)
}

func (d *Driver) releaseAdditionalLayerByID(id string) {
	if al, err := d.getAdditionalLayerPathByID(id); err == nil {
		notifyReleaseAdditionalLayer(al)
	} else if !os.IsNotExist(err) {
		logrus.Warnf("unexpected error on reading Additional Layer Store pointer %v", err)
	}
}

// additionalLayer represents a layer in Additional Layer Store.
type additionalLayer struct {
	path        string
	d           *Driver
	releaseOnce sync.Once
}

// Info returns arbitrary information stored along with this layer (i.e. `info` file).
// This API is experimental and can be changed without bumping the major version number.
// TODO: to remove the comment once it's no longer experimental.
func (al *additionalLayer) Info() (io.ReadCloser, error) {
	return os.Open(filepath.Join(al.path, "info"))
}

// Blob returns a reader of the raw contents of this leyer.
func (al *additionalLayer) Blob() (io.ReadCloser, error) {
	return os.Open(filepath.Join(al.path, "blob"))
}

// CreateAs creates a new layer from this additional layer.
// This API is experimental and can be changed without bumping the major version number.
// TODO: to remove the comment once it's no longer experimental.
func (al *additionalLayer) CreateAs(id, parent string) error {
	// TODO: support opts
	if err := al.d.Create(id, parent, nil); err != nil {
		return err
	}
	dir := al.d.dir(id)
	diffDir := path.Join(dir, "diff")
	if err := os.RemoveAll(diffDir); err != nil {
		return err
	}
	// tell the additional layer store that we use this layer.
	// mark this layer as "additional layer"
	if err := ioutil.WriteFile(path.Join(dir, "additionallayer"), []byte(al.path), 0644); err != nil {
		return err
	}
	notifyUseAdditionalLayer(al.path)
	return os.Symlink(filepath.Join(al.path, "diff"), diffDir)
}

func (d *Driver) getAdditionalLayerPathByID(id string) (string, error) {
	al, err := ioutil.ReadFile(path.Join(d.dir(id), "additionallayer"))
	if err != nil {
		return "", err
	}
	return string(al), nil
}

// Release tells the additional layer store that we don't use this handler.
// This API is experimental and can be changed without bumping the major version number.
// TODO: to remove the comment once it's no longer experimental.
func (al *additionalLayer) Release() {
	// Tell the additional layer store that we don't use this layer handler.
	// This will decrease the reference counter on the store's side, which was
	// increased in LookupAdditionalLayer (so this must be called only once).
	al.releaseOnce.Do(func() {
		notifyReleaseAdditionalLayer(al.path)
	})
}

// notifyUseAdditionalLayer notifies Additional Layer Store that we use the specified layer.
// This is done by creating "use" file in the layer directory. This is useful for
// Additional Layer Store to consider when to perform GC. Notification-aware Additional
// Layer Store must return ENOENT.
func notifyUseAdditionalLayer(al string) {
	if !path.IsAbs(al) {
		logrus.Warnf("additionallayer must be absolute (got: %v)", al)
		return
	}
	useFile := path.Join(al, "use")
	f, err := os.Create(useFile)
	if os.IsNotExist(err) {
		return
	} else if err == nil {
		f.Close()
		if err := os.Remove(useFile); err != nil {
			logrus.Warnf("failed to remove use file")
		}
	}
	logrus.Warnf("unexpected error by Additional Layer Store %v during use; GC doesn't seem to be supported", err)
}

// notifyReleaseAdditionalLayer notifies Additional Layer Store that we don't use the specified
// layer anymore. This is done by rmdir-ing the layer directory. This is useful for
// Additional Layer Store to consider when to perform GC. Notification-aware Additional
// Layer Store must return ENOENT.
func notifyReleaseAdditionalLayer(al string) {
	if !path.IsAbs(al) {
		logrus.Warnf("additionallayer must be absolute (got: %v)", al)
		return
	}
	// tell the additional layer store that we don't use this layer anymore.
	err := unix.Rmdir(al)
	if os.IsNotExist(err) {
		return
	}
	logrus.Warnf("unexpected error by Additional Layer Store %v during release; GC doesn't seem to be supported", err)
}

// redirectDiffIfAdditionalLayer checks if the passed diff path is Additional Layer and
// returns the redirected path. If the passed diff is not the one in Additional Layer
// Store, it returns the original path without changes.
func redirectDiffIfAdditionalLayer(diffPath string) (string, error) {
	if ld, err := os.Readlink(diffPath); err == nil {
		// diff is the link to Additional Layer Store
		if !path.IsAbs(ld) {
			return "", fmt.Errorf("linkpath must be absolute (got: %q)", ld)
		}
		diffPath = ld
	} else if err.(*os.PathError).Err != syscall.EINVAL {
		return "", err
	}
	return diffPath, nil
}
