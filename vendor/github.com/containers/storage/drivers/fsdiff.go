package graphdriver

import (
	"io"
	"time"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	rsystem "github.com/opencontainers/runc/libcontainer/system"
	"github.com/sirupsen/logrus"
)

var (
	// ApplyUncompressedLayer defines the unpack method used by the graph
	// driver.
	ApplyUncompressedLayer = chrootarchive.ApplyUncompressedLayer
)

// NaiveDiffDriver takes a ProtoDriver and adds the
// capability of the Diffing methods which it may or may not
// support on its own. See the comment on the exported
// NewNaiveDiffDriver function below.
// Notably, the AUFS driver doesn't need to be wrapped like this.
type NaiveDiffDriver struct {
	ProtoDriver
	LayerIDMapUpdater
}

// NewNaiveDiffDriver returns a fully functional driver that wraps the
// given ProtoDriver and adds the capability of the following methods which
// it may or may not support on its own:
//     Diff(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (io.ReadCloser, error)
//     Changes(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) ([]archive.Change, error)
//     ApplyDiff(id, parent string, options ApplyDiffOpts) (size int64, err error)
//     DiffSize(id string, idMappings *idtools.IDMappings, parent, parentMappings *idtools.IDMappings, mountLabel string) (size int64, err error)
func NewNaiveDiffDriver(driver ProtoDriver, updater LayerIDMapUpdater) Driver {
	return &NaiveDiffDriver{ProtoDriver: driver, LayerIDMapUpdater: updater}
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (gdw *NaiveDiffDriver) Diff(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (arch io.ReadCloser, err error) {
	startTime := time.Now()
	driver := gdw.ProtoDriver

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}
	if parentMappings == nil {
		parentMappings = &idtools.IDMappings{}
	}

	options := MountOpts{
		MountLabel: mountLabel,
	}
	layerFs, err := driver.Get(id, options)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			driver.Put(id)
		}
	}()

	if parent == "" {
		archive, err := archive.TarWithOptions(layerFs, &archive.TarOptions{
			Compression: archive.Uncompressed,
			UIDMaps:     idMappings.UIDs(),
			GIDMaps:     idMappings.GIDs(),
		})
		if err != nil {
			return nil, err
		}
		return ioutils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			driver.Put(id)
			return err
		}), nil
	}

	options.Options = append(options.Options, "ro")
	parentFs, err := driver.Get(parent, options)
	if err != nil {
		return nil, err
	}
	defer driver.Put(parent)

	changes, err := archive.ChangesDirs(layerFs, idMappings, parentFs, parentMappings)
	if err != nil {
		return nil, err
	}

	archive, err := archive.ExportChanges(layerFs, changes, idMappings.UIDs(), idMappings.GIDs())
	if err != nil {
		return nil, err
	}

	return ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		driver.Put(id)

		// NaiveDiffDriver compares file metadata with parent layers. Parent layers
		// are extracted from tar's with full second precision on modified time.
		// We need this hack here to make sure calls within same second receive
		// correct result.
		time.Sleep(startTime.Truncate(time.Second).Add(time.Second).Sub(time.Now()))
		return err
	}), nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (gdw *NaiveDiffDriver) Changes(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) ([]archive.Change, error) {
	driver := gdw.ProtoDriver

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}
	if parentMappings == nil {
		parentMappings = &idtools.IDMappings{}
	}

	options := MountOpts{
		MountLabel: mountLabel,
	}
	layerFs, err := driver.Get(id, options)
	if err != nil {
		return nil, err
	}
	defer driver.Put(id)

	parentFs := ""

	if parent != "" {
		options := MountOpts{
			MountLabel: mountLabel,
		}
		parentFs, err = driver.Get(parent, options)
		if err != nil {
			return nil, err
		}
		defer driver.Put(parent)
	}

	return archive.ChangesDirs(layerFs, idMappings, parentFs, parentMappings)
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (gdw *NaiveDiffDriver) ApplyDiff(id, parent string, options ApplyDiffOpts) (size int64, err error) {
	driver := gdw.ProtoDriver

	if options.Mappings == nil {
		options.Mappings = &idtools.IDMappings{}
	}

	// Mount the root filesystem so we can apply the diff/layer.
	mountOpts := MountOpts{
		MountLabel: options.MountLabel,
	}
	layerFs, err := driver.Get(id, mountOpts)
	if err != nil {
		return
	}
	defer driver.Put(id)

	tarOptions := &archive.TarOptions{
		InUserNS:          rsystem.RunningInUserNS(),
		IgnoreChownErrors: options.IgnoreChownErrors,
	}
	if options.Mappings != nil {
		tarOptions.UIDMaps = options.Mappings.UIDs()
		tarOptions.GIDMaps = options.Mappings.GIDs()
	}
	start := time.Now().UTC()
	logrus.Debug("Start untar layer")
	if size, err = ApplyUncompressedLayer(layerFs, options.Diff, tarOptions); err != nil {
		logrus.Errorf("Error while applying layer: %s", err)
		return
	}
	logrus.Debugf("Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

	return
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (gdw *NaiveDiffDriver) DiffSize(id string, idMappings *idtools.IDMappings, parent string, parentMappings *idtools.IDMappings, mountLabel string) (size int64, err error) {
	driver := gdw.ProtoDriver

	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}
	if parentMappings == nil {
		parentMappings = &idtools.IDMappings{}
	}

	changes, err := gdw.Changes(id, idMappings, parent, parentMappings, mountLabel)
	if err != nil {
		return
	}

	options := MountOpts{
		MountLabel: mountLabel,
	}
	layerFs, err := driver.Get(id, options)
	if err != nil {
		return
	}
	defer driver.Put(id)

	return archive.ChangesSize(layerFs, changes), nil
}
