//go:build linux
// +build linux

/*

aufs driver directory structure

  .
  ├── layers // Metadata of layers
  │   ├── 1
  │   ├── 2
  │   └── 3
  ├── diff  // Content of the layer
  │   ├── 1  // Contains layers that need to be mounted for the id
  │   ├── 2
  │   └── 3
  └── mnt    // Mount points for the rw layers to be mounted
      ├── 1
      ├── 2
      └── 3

*/

package aufs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	graphdriver "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/directory"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/locker"
	mountpk "github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/parsers"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"github.com/vbatts/tar-split/tar/storage"
	"golang.org/x/sys/unix"
)

var (
	// ErrAufsNotSupported is returned if aufs is not supported by the host.
	ErrAufsNotSupported = fmt.Errorf("aufs was not found in /proc/filesystems")
	// ErrAufsNested means aufs cannot be used bc we are in a user namespace
	ErrAufsNested = fmt.Errorf("aufs cannot be used in non-init user namespace")
	backingFs     = "<unknown>"

	enableDirpermLock sync.Once
	enableDirperm     bool
)

const defaultPerms = os.FileMode(0o555)

func init() {
	graphdriver.MustRegister("aufs", Init)
}

// Driver contains information about the filesystem mounted.
type Driver struct {
	sync.Mutex
	root          string
	uidMaps       []idtools.IDMap
	gidMaps       []idtools.IDMap
	ctr           *graphdriver.RefCounter
	pathCacheLock sync.Mutex
	pathCache     map[string]string
	naiveDiff     graphdriver.DiffDriver
	locker        *locker.Locker
	mountOptions  string
}

// Init returns a new AUFS driver.
// An error is returned if AUFS is not supported.
func Init(home string, options graphdriver.Options) (graphdriver.Driver, error) {
	// Try to load the aufs kernel module
	if err := supportsAufs(); err != nil {
		return nil, fmt.Errorf("kernel does not support aufs: %w", graphdriver.ErrNotSupported)
	}

	fsMagic, err := graphdriver.GetFSMagic(home)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	switch fsMagic {
	case graphdriver.FsMagicAufs, graphdriver.FsMagicBtrfs, graphdriver.FsMagicEcryptfs:
		logrus.Errorf("AUFS is not supported over %s", backingFs)
		return nil, fmt.Errorf("aufs is not supported over %q: %w", backingFs, graphdriver.ErrIncompatibleFS)
	}

	var mountOptions string
	for _, option := range options.DriverOptions {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch key {
		case "aufs.mountopt":
			mountOptions = val
		default:
			return nil, fmt.Errorf("option %s not supported", option)
		}
	}
	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	a := &Driver{
		root:         home,
		uidMaps:      options.UIDMaps,
		gidMaps:      options.GIDMaps,
		pathCache:    make(map[string]string),
		ctr:          graphdriver.NewRefCounter(graphdriver.NewFsChecker(graphdriver.FsMagicAufs)),
		locker:       locker.New(),
		mountOptions: mountOptions,
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(options.UIDMaps, options.GIDMaps)
	if err != nil {
		return nil, err
	}
	// Create the root aufs driver dir and return
	// if it already exists
	// If not populate the dir structure
	if err := idtools.MkdirAllAs(home, 0o700, rootUID, rootGID); err != nil {
		if os.IsExist(err) {
			return a, nil
		}
		return nil, err
	}

	if err := mountpk.MakePrivate(home); err != nil {
		return nil, err
	}

	// Populate the dir structure
	for _, p := range paths {
		if err := idtools.MkdirAllAs(path.Join(home, p), 0o700, rootUID, rootGID); err != nil {
			return nil, err
		}
	}
	logger := logrus.WithFields(logrus.Fields{
		"module": "graphdriver",
		"driver": "aufs",
	})

	for _, path := range []string{"mnt", "diff"} {
		p := filepath.Join(home, path)
		entries, err := os.ReadDir(p)
		if err != nil {
			logger.WithError(err).WithField("dir", p).Error("error reading dir entries")
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if strings.HasSuffix(entry.Name(), "-removing") {
				logger.WithField("dir", entry.Name()).Debug("Cleaning up stale layer dir")
				if err := system.EnsureRemoveAll(filepath.Join(p, entry.Name())); err != nil {
					logger.WithField("dir", entry.Name()).WithError(err).Error("Error removing stale layer dir")
				}
			}
		}
	}

	a.naiveDiff = graphdriver.NewNaiveDiffDriver(a, a)
	return a, nil
}

// Return a nil error if the kernel supports aufs
// We cannot modprobe because inside dind modprobe fails
// to run
func supportsAufs() error {
	// We can try to modprobe aufs first before looking at
	// proc/filesystems for when aufs is supported
	exec.Command("modprobe", "aufs").Run()

	if unshare.IsRootless() {
		return ErrAufsNested
	}

	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), "aufs") {
			return nil
		}
	}
	return ErrAufsNotSupported
}

func (a *Driver) rootPath() string {
	return a.root
}

func (*Driver) String() string {
	return "aufs"
}

// Status returns current information about the filesystem such as root directory, number of directories mounted, etc.
func (a *Driver) Status() [][2]string {
	ids, _ := loadIds(path.Join(a.rootPath(), "layers"))
	return [][2]string{
		{"Root Dir", a.rootPath()},
		{"Backing Filesystem", backingFs},
		{"Dirs", fmt.Sprintf("%d", len(ids))},
		{"Dirperm1 Supported", fmt.Sprintf("%v", useDirperm())},
	}
}

// Metadata not implemented
func (a *Driver) Metadata(id string) (map[string]string, error) {
	return nil, nil //nolint: nilnil
}

// Exists returns true if the given id is registered with
// this driver
func (a *Driver) Exists(id string) bool {
	if _, err := os.Lstat(path.Join(a.rootPath(), "layers", id)); err != nil {
		return false
	}
	return true
}

// ListLayers() returns all of the layers known to the driver.
func (a *Driver) ListLayers() ([]string, error) {
	diffsDir := filepath.Join(a.rootPath(), "diff")
	entries, err := os.ReadDir(diffsDir)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		results = append(results, entry.Name())
	}
	return results, nil
}

// AdditionalImageStores returns additional image stores supported by the driver
func (a *Driver) AdditionalImageStores() []string {
	return nil
}

// CreateFromTemplate creates a layer with the same contents and parent as another layer.
func (a *Driver) CreateFromTemplate(id, template string, templateIDMappings *idtools.IDMappings, parent string, parentIDMappings *idtools.IDMappings, opts *graphdriver.CreateOpts, readWrite bool) error {
	if opts == nil {
		opts = &graphdriver.CreateOpts{}
	}
	return graphdriver.NaiveCreateFromTemplate(a, id, template, templateIDMappings, parent, parentIDMappings, opts, readWrite)
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (a *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	return a.Create(id, parent, opts)
}

// Create three folders for each id
// mnt, layers, and diff
func (a *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	if opts != nil && len(opts.StorageOpt) != 0 {
		return fmt.Errorf("--storage-opt is not supported for aufs")
	}

	if err := a.createDirsFor(id, parent); err != nil {
		return err
	}
	// Write the layers metadata
	f, err := os.Create(path.Join(a.rootPath(), "layers", id))
	if err != nil {
		return err
	}
	defer f.Close()

	if parent != "" {
		ids, err := getParentIDs(a.rootPath(), parent)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintln(f, parent); err != nil {
			return err
		}
		for _, i := range ids {
			if _, err := fmt.Fprintln(f, i); err != nil {
				return err
			}
		}
	}

	return nil
}

// createDirsFor creates two directories for the given id.
// mnt and diff
func (a *Driver) createDirsFor(id, parent string) error {
	paths := []string{
		"mnt",
		"diff",
	}

	// Directory permission is 0555.
	// The path of directories are <aufs_root_path>/mnt/<image_id>
	// and <aufs_root_path>/diff/<image_id>
	for _, p := range paths {
		rootPair := idtools.NewIDMappingsFromMaps(a.uidMaps, a.gidMaps).RootPair()
		rootPerms := defaultPerms
		if parent != "" {
			st, err := system.Stat(path.Join(a.rootPath(), p, parent))
			if err != nil {
				return err
			}
			rootPerms = os.FileMode(st.Mode())
			rootPair.UID = int(st.UID())
			rootPair.GID = int(st.GID())
		}
		if err := idtools.MkdirAllAndChownNew(path.Join(a.rootPath(), p, id), rootPerms, rootPair); err != nil {
			return err
		}
	}
	return nil
}

// Remove will unmount and remove the given id.
func (a *Driver) Remove(id string) error {
	a.locker.Lock(id)
	defer a.locker.Unlock(id)
	a.pathCacheLock.Lock()
	mountpoint, exists := a.pathCache[id]
	a.pathCacheLock.Unlock()
	if !exists {
		mountpoint = a.getMountpoint(id)
	}

	logger := logrus.WithFields(logrus.Fields{
		"module": "graphdriver",
		"driver": "aufs",
		"layer":  id,
	})

	var retries int
	for {
		mounted, err := a.mounted(mountpoint)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return err
		}
		if !mounted {
			break
		}

		err = a.unmount(mountpoint)
		if err == nil {
			break
		}

		if err != unix.EBUSY {
			return fmt.Errorf("aufs: unmount error: %s: %w", mountpoint, err)
		}
		if retries >= 5 {
			return fmt.Errorf("aufs: unmount error after retries: %s: %w", mountpoint, err)
		}
		// If unmount returns EBUSY, it could be a transient error. Sleep and retry.
		retries++
		logger.Warnf("unmount failed due to EBUSY: retry count: %d", retries)
		time.Sleep(100 * time.Millisecond)
	}

	// Remove the layers file for the id
	if err := os.Remove(path.Join(a.rootPath(), "layers", id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing layers dir for %s: %w", id, err)
	}

	if err := atomicRemove(a.getDiffPath(id)); err != nil {
		return fmt.Errorf("could not remove diff path for id %s: %w", id, err)
	}

	// Atomically remove each directory in turn by first moving it out of the
	// way (so that container runtime doesn't find it anymore) before doing removal of
	// the whole tree.
	if err := atomicRemove(mountpoint); err != nil {
		if errors.Is(err, unix.EBUSY) {
			logger.WithField("dir", mountpoint).WithError(err).Warn("error performing atomic remove due to EBUSY")
		}
		return fmt.Errorf("could not remove mountpoint for id %s: %w", id, err)
	}

	a.pathCacheLock.Lock()
	delete(a.pathCache, id)
	a.pathCacheLock.Unlock()
	return nil
}

func atomicRemove(source string) error {
	target := source + "-removing"

	err := os.Rename(source, target)
	switch {
	case err == nil, os.IsNotExist(err):
	case os.IsExist(err):
		// Got error saying the target dir already exists, maybe the source doesn't exist due to a previous (failed) remove
		if _, e := os.Stat(source); !os.IsNotExist(e) {
			return fmt.Errorf("target rename dir '%s' exists but should not, this needs to be manually cleaned up: %w", target, err)
		}
	default:
		return fmt.Errorf("preparing atomic delete: %w", err)
	}

	return system.EnsureRemoveAll(target)
}

// Get returns the rootfs path for the id.
// This will mount the dir at its given path
func (a *Driver) Get(id string, options graphdriver.MountOpts) (string, error) {
	a.locker.Lock(id)
	defer a.locker.Unlock(id)
	parents, err := a.getParentLayerPaths(id)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	a.pathCacheLock.Lock()
	m, exists := a.pathCache[id]
	a.pathCacheLock.Unlock()

	if !exists {
		m = a.getDiffPath(id)
		if len(parents) > 0 {
			m = a.getMountpoint(id)
		}
	}
	if count := a.ctr.Increment(m); count > 1 {
		return m, nil
	}

	// If a dir does not have a parent ( no layers )do not try to mount
	// just return the diff path to the data
	if len(parents) > 0 {
		if err := a.mount(id, m, parents, options); err != nil {
			return "", err
		}
	}

	a.pathCacheLock.Lock()
	a.pathCache[id] = m
	a.pathCacheLock.Unlock()
	return m, nil
}

// Put unmounts and updates list of active mounts.
func (a *Driver) Put(id string) error {
	a.locker.Lock(id)
	defer a.locker.Unlock(id)
	a.pathCacheLock.Lock()
	m, exists := a.pathCache[id]
	if !exists {
		m = a.getMountpoint(id)
		a.pathCache[id] = m
	}
	a.pathCacheLock.Unlock()
	if count := a.ctr.Decrement(m); count > 0 {
		return nil
	}

	err := a.unmount(m)
	if err != nil {
		logrus.Debugf("Failed to unmount %s aufs: %v", id, err)
	}
	return err
}

// ReadWriteDiskUsage returns the disk usage of the writable directory for the ID.
// For AUFS, it queries the mountpoint for this ID.
func (a *Driver) ReadWriteDiskUsage(id string) (*directory.DiskUsage, error) {
	a.locker.Lock(id)
	defer a.locker.Unlock(id)
	a.pathCacheLock.Lock()
	m, exists := a.pathCache[id]
	if !exists {
		m = a.getMountpoint(id)
		a.pathCache[id] = m
	}
	a.pathCacheLock.Unlock()
	return directory.Usage(m)
}

// isParent returns if the passed in parent is the direct parent of the passed in layer
func (a *Driver) isParent(id, parent string) bool {
	parents, _ := getParentIDs(a.rootPath(), id)
	if parent == "" && len(parents) > 0 {
		return false
	}
	return !(len(parents) > 0 && parent != parents[0])
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (a *Driver) Diff(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (io.ReadCloser, error) {
	if !a.isParent(id, parent) {
		return a.naiveDiff.Diff(id, idMappings, parent, parentMappings, mountLabel)
	}

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}

	// AUFS doesn't need the parent layer to produce a diff.
	return archive.TarWithOptions(path.Join(a.rootPath(), "diff", id), &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: []string{archive.WhiteoutMetaPrefix + "*", "!" + archive.WhiteoutOpaqueDir},
		UIDMaps:         idMappings.UIDs(),
		GIDMaps:         idMappings.GIDs(),
	})
}

type fileGetNilCloser struct {
	storage.FileGetter
}

func (f fileGetNilCloser) Close() error {
	return nil
}

// DiffGetter returns a FileGetCloser that can read files from the directory that
// contains files for the layer differences. Used for direct access for tar-split.
func (a *Driver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	p := path.Join(a.rootPath(), "diff", id)
	return fileGetNilCloser{storage.NewPathFileGetter(p)}, nil
}

func (a *Driver) applyDiff(id string, idMappings *idtools.IDMappings, diff io.Reader) error {
	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}
	return chrootarchive.UntarUncompressed(diff, path.Join(a.rootPath(), "diff", id), &archive.TarOptions{
		UIDMaps: idMappings.UIDs(),
		GIDMaps: idMappings.GIDs(),
	})
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (a *Driver) DiffSize(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (size int64, err error) {
	if !a.isParent(id, parent) {
		return a.naiveDiff.DiffSize(id, idMappings, parent, parentMappings, mountLabel)
	}
	// AUFS doesn't need the parent layer to calculate the diff size.
	return directory.Size(path.Join(a.rootPath(), "diff", id))
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (a *Driver) ApplyDiff(id, parent string, options graphdriver.ApplyDiffOpts) (size int64, err error) {
	if !a.isParent(id, parent) {
		return a.naiveDiff.ApplyDiff(id, parent, options)
	}

	// AUFS doesn't need the parent id to apply the diff if it is the direct parent.
	if err = a.applyDiff(id, options.Mappings, options.Diff); err != nil {
		return
	}

	return directory.Size(path.Join(a.rootPath(), "diff", id))
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (a *Driver) Changes(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) ([]archive.Change, error) {
	if !a.isParent(id, parent) {
		return a.naiveDiff.Changes(id, idMappings, parent, parentMappings, mountLabel)
	}

	// AUFS doesn't have snapshots, so we need to get changes from all parent
	// layers.
	layers, err := a.getParentLayerPaths(id)
	if err != nil {
		return nil, err
	}
	return archive.Changes(layers, path.Join(a.rootPath(), "diff", id))
}

func (a *Driver) getParentLayerPaths(id string) ([]string, error) {
	parentIds, err := getParentIDs(a.rootPath(), id)
	if err != nil {
		return nil, err
	}
	layers := make([]string, len(parentIds))

	// Get the diff paths for all the parent ids
	for i, p := range parentIds {
		layers[i] = path.Join(a.rootPath(), "diff", p)
	}
	return layers, nil
}

func (a *Driver) mount(id string, target string, layers []string, options graphdriver.MountOpts) error {
	a.Lock()
	defer a.Unlock()

	// If the id is mounted or we get an error return
	if mounted, err := a.mounted(target); err != nil || mounted {
		return err
	}

	rw := a.getDiffPath(id)

	if err := a.aufsMount(layers, rw, target, options); err != nil {
		return fmt.Errorf("creating aufs mount to %s: %w", target, err)
	}
	return nil
}

func (a *Driver) unmount(mountPath string) error {
	a.Lock()
	defer a.Unlock()

	if mounted, err := a.mounted(mountPath); err != nil || !mounted {
		return err
	}
	if err := Unmount(mountPath); err != nil {
		return err
	}
	return nil
}

func (a *Driver) mounted(mountpoint string) (bool, error) {
	return graphdriver.Mounted(graphdriver.FsMagicAufs, mountpoint)
}

// Cleanup aufs and unmount all mountpoints
func (a *Driver) Cleanup() error {
	var dirs []string
	if err := filepath.WalkDir(a.mntPath(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		dirs = append(dirs, path)
		return nil
	}); err != nil {
		return err
	}

	for _, m := range dirs {
		if err := a.unmount(m); err != nil {
			logrus.Debugf("aufs error unmounting %s: %s", m, err)
		}
	}
	return mountpk.Unmount(a.root)
}

func (a *Driver) aufsMount(ro []string, rw, target string, options graphdriver.MountOpts) (err error) {
	defer func() {
		if err != nil {
			Unmount(target)
		}
	}()

	// Mount options are clipped to page size(4096 bytes). If there are more
	// layers then these are remounted individually using append.

	offset := 54
	if useDirperm() {
		offset += len(",dirperm1")
	}
	b := make([]byte, unix.Getpagesize()-len(options.MountLabel)-offset) // room for xino & mountLabel
	bp := copy(b, fmt.Sprintf("br:%s=rw", rw))

	index := 0
	for ; index < len(ro); index++ {
		layer := fmt.Sprintf(":%s=ro+wh", ro[index])
		if bp+len(layer) > len(b) {
			break
		}
		bp += copy(b[bp:], layer)
	}

	opts := "dio,xino=/dev/shm/aufs.xino"
	mountOptions := a.mountOptions
	if len(options.Options) > 0 {
		mountOptions = strings.Join(options.Options, ",")
	}
	if mountOptions != "" {
		opts += fmt.Sprintf(",%s", mountOptions)
	}

	if useDirperm() {
		opts += ",dirperm1"
	}
	data := label.FormatMountLabel(fmt.Sprintf("%s,%s", string(b[:bp]), opts), options.MountLabel)
	if err = mount("none", target, "aufs", 0, data); err != nil {
		return
	}

	for ; index < len(ro); index++ {
		layer := fmt.Sprintf(":%s=ro+wh", ro[index])
		data := label.FormatMountLabel(fmt.Sprintf("append%s", layer), options.MountLabel)
		if err = mount("none", target, "aufs", unix.MS_REMOUNT, data); err != nil {
			return
		}
	}

	return
}

// useDirperm checks dirperm1 mount option can be used with the current
// version of aufs.
func useDirperm() bool {
	enableDirpermLock.Do(func() {
		base, err := os.MkdirTemp("", "storage-aufs-base")
		if err != nil {
			logrus.Errorf("Checking dirperm1: %v", err)
			return
		}
		defer os.RemoveAll(base)

		union, err := os.MkdirTemp("", "storage-aufs-union")
		if err != nil {
			logrus.Errorf("Checking dirperm1: %v", err)
			return
		}
		defer os.RemoveAll(union)

		opts := fmt.Sprintf("br:%s,dirperm1,xino=/dev/shm/aufs.xino", base)
		if err := mount("none", union, "aufs", 0, opts); err != nil {
			return
		}
		enableDirperm = true
		if err := Unmount(union); err != nil {
			logrus.Errorf("Checking dirperm1: failed to unmount %v", err)
		}
	})
	return enableDirperm
}

// UpdateLayerIDMap updates ID mappings in a layer from matching the ones
// specified by toContainer to those specified by toHost.
func (a *Driver) UpdateLayerIDMap(id string, toContainer, toHost *idtools.IDMappings, mountLabel string) error {
	return fmt.Errorf("aufs doesn't support changing ID mappings")
}

// SupportsShifting tells whether the driver support shifting of the UIDs/GIDs in an userNS
func (a *Driver) SupportsShifting() bool {
	return false
}
