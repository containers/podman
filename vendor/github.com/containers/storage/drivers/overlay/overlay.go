// +build linux

package overlay

import (
	"bytes"
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
	"github.com/containers/storage/pkg/ostree"
	"github.com/containers/storage/pkg/parsers"
	"github.com/containers/storage/pkg/system"
	units "github.com/docker/go-units"
	rsystem "github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	// untar defines the untar method
	untar = chrootarchive.UntarUncompressed
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
	// which can be used to create the unique link identifer
	// for every layer. If this value is too long then the
	// page size limit for the mount command may be exceeded.
	// The idLength should be selected such that following equation
	// is true (512 is a buffer for label metadata).
	// ((idLength + len(linkDir) + 1) * maxDepth) <= (pageSize - 512)
	idLength = 26
)

type overlayOptions struct {
	imageStores   []string
	quota         quota.Quota
	mountProgram  string
	ostreeRepo    string
	skipMountHome bool
	mountOptions  string
}

// Driver contains information about the home directory and the list of active mounts that are created using this driver.
type Driver struct {
	name          string
	home          string
	runhome       string
	uidMaps       []idtools.IDMap
	gidMaps       []idtools.IDMap
	ctr           *graphdriver.RefCounter
	quotaCtl      *quota.Control
	options       overlayOptions
	naiveDiff     graphdriver.DiffDriver
	supportsDType bool
	usingMetacopy bool
	locker        *locker.Locker
	convert       map[string]bool
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

	// check if they are running over btrfs, aufs, zfs, overlay, or ecryptfs
	if opts.mountProgram == "" {
		switch fsMagic {
		case graphdriver.FsMagicAufs, graphdriver.FsMagicZfs, graphdriver.FsMagicOverlay, graphdriver.FsMagicEcryptfs:
			logrus.Errorf("'overlay' is not supported over %s", backingFs)
			return nil, errors.Wrapf(graphdriver.ErrIncompatibleFS, "'overlay' is not supported over %s", backingFs)
		}
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(options.UIDMaps, options.GIDMaps)
	if err != nil {
		return nil, err
	}

	// Create the driver home dir
	if err := idtools.MkdirAllAs(path.Join(home, linkDir), 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}
	runhome := filepath.Join(options.RunRoot, filepath.Base(home))
	if err := idtools.MkdirAllAs(runhome, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	var usingMetacopy bool
	var supportsDType bool
	if opts.mountProgram != "" {
		supportsDType = true
	} else {
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
				return nil, errors.New(overlayCacheText)
			}
		} else {
			supportsDType, err = supportsOverlay(home, fsMagic, rootUID, rootGID)
			if err != nil {
				os.Remove(filepath.Join(home, linkDir))
				os.Remove(home)
				patherr, ok := err.(*os.PathError)
				if ok && patherr.Err == syscall.ENOSPC {
					return nil, err
				}
				err = errors.Wrap(err, "kernel does not support overlay fs")
				if err2 := cachedFeatureRecord(runhome, feature, false, err.Error()); err2 != nil {
					return nil, errors.Wrapf(err2, "error recording overlay not being supported (%v)", err)
				}
				return nil, err
			}
			if err = cachedFeatureRecord(runhome, feature, supportsDType, ""); err != nil {
				return nil, errors.Wrap(err, "error recording overlay support status")
			}
		}

		feature = fmt.Sprintf("metacopy(%s)", opts.mountOptions)
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
				logrus.Warnf("overlay test mount did not indicate whether or not metacopy is being used: %v", err)
				return nil, err
			}
		}
	}

	if !opts.skipMountHome {
		if err := mount.MakePrivate(home); err != nil {
			return nil, err
		}
	}

	if opts.ostreeRepo != "" {
		if err := ostree.CreateOSTreeRepository(opts.ostreeRepo, rootUID, rootGID); err != nil {
			return nil, err
		}
	}

	d := &Driver{
		name:          "overlay",
		home:          home,
		runhome:       runhome,
		uidMaps:       options.UIDMaps,
		gidMaps:       options.GIDMaps,
		ctr:           graphdriver.NewRefCounter(graphdriver.NewFsChecker(graphdriver.FsMagicOverlay)),
		supportsDType: supportsDType,
		usingMetacopy: usingMetacopy,
		locker:        locker.New(),
		options:       *opts,
		convert:       make(map[string]bool),
	}

	d.naiveDiff = graphdriver.NewNaiveDiffDriver(d, d)

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
		key = strings.ToLower(key)
		switch key {
		case ".override_kernel_check", "overlay.override_kernel_check", "overlay2.override_kernel_check":
			logrus.Warnf("overlay: override_kernel_check option was specified, but is no longer necessary")
		case ".mountopt", "overlay.mountopt", "overlay2.mountopt":
			o.mountOptions = val
		case ".size", "overlay.size", "overlay2.size":
			logrus.Debugf("overlay: size=%s", val)
			size, err := units.RAMInBytes(val)
			if err != nil {
				return nil, err
			}
			o.quota.Size = uint64(size)
		case ".imagestore", "overlay.imagestore", "overlay2.imagestore":
			logrus.Debugf("overlay: imagestore=%s", val)
			// Additional read only image stores to use for lower paths
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
		case ".mount_program", "overlay.mount_program", "overlay2.mount_program":
			logrus.Debugf("overlay: mount_program=%s", val)
			_, err := os.Stat(val)
			if err != nil {
				return nil, fmt.Errorf("overlay: can't stat program %s: %v", val, err)
			}
			o.mountProgram = val
		case "overlay2.ostree_repo", "overlay.ostree_repo", ".ostree_repo":
			logrus.Debugf("overlay: ostree_repo=%s", val)
			if !ostree.OstreeSupport() {
				return nil, fmt.Errorf("overlay: ostree_repo specified but support for ostree is missing")
			}
			o.ostreeRepo = val
		case "overlay2.skip_mount_home", "overlay.skip_mount_home", ".skip_mount_home":
			logrus.Debugf("overlay: skip_mount_home=%s", val)
			o.skipMountHome, err = strconv.ParseBool(val)
			if err != nil {
				return nil, err
			}
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

func supportsOverlay(home string, homeMagic graphdriver.FsMagic, rootUID, rootGID int) (supportsDType bool, err error) {
	// We can try to modprobe overlay first

	exec.Command("modprobe", "overlay").Run()

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
		if len(flags) < unix.Getpagesize() {
			err := mountFrom(filepath.Dir(home), "overlay", mergedDir, "overlay", 0, flags)
			if err == nil {
				logrus.Debugf("overlay test mount with multiple lowers succeeded")
				return supportsDType, nil
			} else {
				logrus.Debugf("overlay test mount with multiple lowers failed %v", err)
			}
		}
		flags = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower1Dir, upperDir, workDir)
		if len(flags) < unix.Getpagesize() {
			err := mountFrom(filepath.Dir(home), "overlay", mergedDir, "overlay", 0, flags)
			if err == nil {
				logrus.Errorf("overlay test mount with multiple lowers failed, but succeeded with a single lower")
				return supportsDType, errors.Wrap(graphdriver.ErrNotSupported, "kernel too old to provide multiple lowers feature for overlay")
			} else {
				logrus.Debugf("overlay test mount with a single lower failed %v", err)
			}
		}
		logrus.Errorf("'overlay' is not supported over %s at %q", backingFs, home)
		return supportsDType, errors.Wrapf(graphdriver.ErrIncompatibleFS, "'overlay' is not supported over %s at %q", backingFs, home)
	}

	logrus.Error("'overlay' not found as a supported filesystem on this host. Please ensure kernel is new enough and has overlay support loaded.")
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
				logrus.Warn(nativeDiffCacheText)
			}
			useNaiveDiffOnly = !nativeDiffCacheResult
			return
		}
		if err := doesSupportNativeDiff(d.home, d.options.mountOptions); err != nil {
			nativeDiffCacheText = fmt.Sprintf("Not using native diff for overlay, this may cause degraded performance for building images: %v", err)
			logrus.Warn(nativeDiffCacheText)
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
	return mount.Unmount(d.home)
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

	if d.options.ostreeRepo != "" {
		d.convert[id] = true
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
	if err := idtools.MkdirAllAs(path.Join(d.home, linkDir), 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
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

	if opts != nil && len(opts.StorageOpt) > 0 {
		driver := &Driver{}
		if err := d.parseStorageOpt(opts.StorageOpt, driver); err != nil {
			return err
		}

		if driver.options.quota.Size > 0 {
			// Set container disk quota limit
			if err := d.quotaCtl.SetQuota(dir, driver.options.quota); err != nil {
				return err
			}
		}
	}

	if err := idtools.MkdirAs(path.Join(dir, "diff"), 0755, rootUID, rootGID); err != nil {
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
		return "", err
	}
	lowers := []string{path.Join(linkDir, string(parentLink))}

	parentLower, err := ioutil.ReadFile(path.Join(parentDir, lowerFile))
	if err == nil {
		parentLowers := strings.Split(string(parentLower), ":")
		lowers = append(lowers, parentLowers...)
	}
	if len(lowers) > maxDepth {
		return "", errors.New("max depth exceeded")
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
			if err != nil {
				return nil, err
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
		for _, i := range uidMaps {
			if uids.Len() > 0 {
				uids.WriteString(":")
			}
			uids.WriteString(fmt.Sprintf("%d:%d:%d", i.ContainerID, i.HostID, i.Size))
		}
		for _, i := range gidMaps {
			if gids.Len() > 0 {
				gids.WriteString(":")
			}
			gids.WriteString(fmt.Sprintf("%d:%d:%d", i.ContainerID, i.HostID, i.Size))
		}
		return fmt.Sprintf("%s,uidmapping=%s,gidmapping=%s", opts, uids.String(), gids.String())
	}
	return opts
}

// Remove cleans the directories that are created for this id.
func (d *Driver) Remove(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)

	// Ignore errors, we don't want to fail if the ostree branch doesn't exist,
	if d.options.ostreeRepo != "" {
		ostree.DeleteOSTree(d.options.ostreeRepo, id)
	}

	dir := d.dir(id)
	lid, err := ioutil.ReadFile(path.Join(dir, "link"))
	if err == nil {
		if err := os.RemoveAll(path.Join(d.home, linkDir, string(lid))); err != nil {
			logrus.Debugf("Failed to remove link: %v", err)
		}
	}

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
	// This makes the link directory if it doesn't exist
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	if err := idtools.MkdirAllAs(path.Join(d.home, linkDir), 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return err
	}
	for _, dir := range dirs {
		// Skip over the linkDir and anything that is not a directory
		if dir.Name() == linkDir || !dir.Mode().IsDir() {
			continue
		}
		// Read the "link" file under each layer to get the name of the symlink
		data, err := ioutil.ReadFile(path.Join(d.dir(dir.Name()), "link"))
		if err != nil {
			return fmt.Errorf("error reading name of symlink for %q: %v", dir, err)
		}
		linkPath := path.Join(d.home, linkDir, strings.Trim(string(data), "\n"))
		// Check if the symlink exists, and if it doesn't create it again with the name we
		// got from the "link" file
		_, err = os.Stat(linkPath)
		if err != nil && os.IsNotExist(err) {
			if err := os.Symlink(path.Join("..", dir.Name(), "diff"), linkPath); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("error trying to stat %q: %v", linkPath, err)
		}
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

	diffDir := path.Join(dir, "diff")
	lowers, err := ioutil.ReadFile(path.Join(dir, lowerFile))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// absLowers is the list of lowers as absolute paths, which works well with additional stores.
	absLowers := []string{}
	// relLowers is the list of lowers as paths relative to the driver's home directory.
	relLowers := []string{}

	// Check if $link/../diff{1-*} exist.  If they do, add them, in order, as the front of the lowers
	// lists that we're building.  "diff" itself is the upper, so it won't be in the lists.
	link, err := ioutil.ReadFile(path.Join(dir, "link"))
	if err != nil {
		return "", err
	}
	diffN := 1
	_, err = os.Stat(filepath.Join(dir, nameWithSuffix("diff", diffN)))
	for err == nil {
		absLowers = append(absLowers, filepath.Join(dir, nameWithSuffix("diff", diffN)))
		relLowers = append(relLowers, dumbJoin(string(link), "..", nameWithSuffix("diff", diffN)))
		diffN++
		_, err = os.Stat(filepath.Join(dir, nameWithSuffix("diff", diffN)))
	}

	// For each lower, resolve its path, and append it and any additional diffN
	// directories to the lowers list.
	for _, l := range strings.Split(string(lowers), ":") {
		if l == "" {
			continue
		}
		lower := ""
		newpath := path.Join(d.home, l)
		if _, err := os.Stat(newpath); err != nil {
			for _, p := range d.AdditionalImageStores() {
				lower = path.Join(p, d.name, l)
				if _, err2 := os.Stat(lower); err2 == nil {
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

	// If the lowers list is still empty, use an empty lower so that we can still force an
	// SELinux context for the mount.
	if len(absLowers) == 0 {
		absLowers = append(absLowers, path.Join(dir, "empty"))
		relLowers = append(relLowers, path.Join(id, "empty"))
	}

	mergedDir := path.Join(dir, "merged")
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

	workDir := path.Join(dir, "work")
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(absLowers, ":"), diffDir, workDir)
	if len(options.Options) > 0 {
		opts = fmt.Sprintf("%s,%s", strings.Join(options.Options, ","), opts)
	} else if d.options.mountOptions != "" {
		opts = fmt.Sprintf("%s,%s", d.options.mountOptions, opts)
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
		//FIXME: We need to figure out to get this to work with additional stores
		opts = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(relLowers, ":"), path.Join(id, "diff"), path.Join(id, "work"))
		mountData = label.FormatMountLabel(opts, options.MountLabel)
		if len(mountData) > pageSize {
			return "", fmt.Errorf("cannot mount layer, mount label too large %d", len(mountData))
		}
		mountFunc = func(source string, target string, mType string, flags uintptr, label string) error {
			return mountFrom(d.home, source, target, mType, flags, label)
		}
		mountTarget = path.Join(id, "merged")
	}
	flags, data := mount.ParseOptions(mountData)
	logrus.Debugf("overlay: mount_data=%s", mountData)
	if err := mountFunc("overlay", mountTarget, "overlay", uintptr(flags), data); err != nil {
		return "", fmt.Errorf("error creating overlay mount to %s: %v", mountTarget, err)
	}

	// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
	// user namespace requires this to move a directory from lower to upper.
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return "", err
	}

	if err := os.Chown(path.Join(workDir, "work"), rootUID, rootGID); err != nil {
		return "", err
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
	mountpoint := path.Join(d.dir(id), "merged")
	if count := d.ctr.Decrement(mountpoint); count > 0 {
		return nil
	}
	if _, err := ioutil.ReadFile(path.Join(dir, lowerFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := unix.Unmount(mountpoint, unix.MNT_DETACH); err != nil {
		logrus.Debugf("Failed to unmount %s overlay: %s - %v", id, mountpoint, err)
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

// ApplyDiff applies the new layer into a root
func (d *Driver) ApplyDiff(id string, idMappings *idtools.IDMappings, parent string, mountLabel string, diff io.Reader) (size int64, err error) {
	if !d.isParent(id, parent) {
		return d.naiveDiff.ApplyDiff(id, idMappings, parent, mountLabel, diff)
	}

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}

	applyDir := d.getDiffPath(id)

	logrus.Debugf("Applying tar in %s", applyDir)
	// Overlay doesn't need the parent id to apply the diff
	if err := untar(diff, applyDir, &archive.TarOptions{
		UIDMaps:        idMappings.UIDs(),
		GIDMaps:        idMappings.GIDs(),
		WhiteoutFormat: d.getWhiteoutFormat(),
		InUserNS:       rsystem.RunningInUserNS(),
	}); err != nil {
		return 0, err
	}

	_, convert := d.convert[id]
	if convert {
		if err := ostree.ConvertToOSTree(d.options.ostreeRepo, applyDir, id); err != nil {
			return 0, err
		}
	}

	return directory.Size(applyDir)
}

func (d *Driver) getDiffPath(id string) string {
	dir := d.dir(id)

	return path.Join(dir, "diff")
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (size int64, err error) {
	if d.useNaiveDiff() || !d.isParent(id, parent) {
		return d.naiveDiff.DiffSize(id, idMappings, parent, parentMappings, mountLabel)
	}
	return directory.Size(d.getDiffPath(id))
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

	lowerDirs, err := d.getLowerDirs(id)
	if err != nil {
		return nil, err
	}

	diffPath := d.getDiffPath(id)
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
	diffPath := d.getDiffPath(id)
	layers, err := d.getLowerDirs(id)
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
	_, err = os.Stat(nameWithSuffix(diffDir, i))
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

	// Re-create the directory that we're going to use as the upper layer.
	if err := idtools.MkdirAs(diffDir, 0755, rootUID, rootGID); err != nil {
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
