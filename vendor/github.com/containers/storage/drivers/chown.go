package graphdriver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/reexec"
)

const (
	chownByMapsCmd = "storage-chown-by-maps"
)

func init() {
	reexec.Register(chownByMapsCmd, chownByMapsMain)
}

func chownByMapsMain() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "requires mapping configuration on stdin and directory path")
		os.Exit(1)
	}
	// Read and decode our configuration.
	discreteMaps := [4][]idtools.IDMap{}
	config := bytes.Buffer{}
	if _, err := config.ReadFrom(os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "error reading configuration: %v", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(config.Bytes(), &discreteMaps); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding configuration: %v", err)
		os.Exit(1)
	}
	// Try to chroot.  This may not be possible, and on some systems that
	// means we just Chdir() to the directory, so from here on we should be
	// using relative paths.
	if err := chrootOrChdir(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "error chrooting to %q: %v", os.Args[1], err)
		os.Exit(1)
	}
	// Build the mapping objects.
	toContainer := idtools.NewIDMappingsFromMaps(discreteMaps[0], discreteMaps[1])
	if len(toContainer.UIDs()) == 0 && len(toContainer.GIDs()) == 0 {
		toContainer = nil
	}
	toHost := idtools.NewIDMappingsFromMaps(discreteMaps[2], discreteMaps[3])
	if len(toHost.UIDs()) == 0 && len(toHost.GIDs()) == 0 {
		toHost = nil
	}
	chown := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %q: %v", path, err)
		}
		sysinfo := info.Sys()
		if st, ok := sysinfo.(*syscall.Stat_t); ok {
			// Map an on-disk UID/GID pair from host to container
			// using the first map, then back to the host using the
			// second map.  Skip that first step if they're 0, to
			// compensate for cases where a parent layer should
			// have had a mapped value, but didn't.
			uid, gid := int(st.Uid), int(st.Gid)
			if toContainer != nil {
				pair := idtools.IDPair{
					UID: uid,
					GID: gid,
				}
				mappedUid, mappedGid, err := toContainer.ToContainer(pair)
				if err != nil {
					if (uid != 0) || (gid != 0) {
						return fmt.Errorf("error mapping host ID pair %#v for %q to container: %v", pair, path, err)
					}
					mappedUid, mappedGid = uid, gid
				}
				uid, gid = mappedUid, mappedGid
			}
			if toHost != nil {
				pair := idtools.IDPair{
					UID: uid,
					GID: gid,
				}
				mappedPair, err := toHost.ToHost(pair)
				if err != nil {
					return fmt.Errorf("error mapping container ID pair %#v for %q to host: %v", pair, path, err)
				}
				uid, gid = mappedPair.UID, mappedPair.GID
			}
			if uid != int(st.Uid) || gid != int(st.Gid) {
				// Make the change.
				if err := syscall.Lchown(path, uid, gid); err != nil {
					return fmt.Errorf("%s: chown(%q): %v", os.Args[0], path, err)
				}
			}
		}
		return nil
	}
	if err := filepath.Walk(".", chown); err != nil {
		fmt.Fprintf(os.Stderr, "error during chown: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// ChownPathByMaps walks the filesystem tree, changing the ownership
// information using the toContainer and toHost mappings, using them to replace
// on-disk owner UIDs and GIDs which are "host" values in the first map with
// UIDs and GIDs for "host" values from the second map which correspond to the
// same "container" IDs.
func ChownPathByMaps(path string, toContainer, toHost *idtools.IDMappings) error {
	if toContainer == nil {
		toContainer = &idtools.IDMappings{}
	}
	if toHost == nil {
		toHost = &idtools.IDMappings{}
	}

	config, err := json.Marshal([4][]idtools.IDMap{toContainer.UIDs(), toContainer.GIDs(), toHost.UIDs(), toHost.GIDs()})
	if err != nil {
		return err
	}
	cmd := reexec.Command(chownByMapsCmd, path)
	cmd.Stdin = bytes.NewReader(config)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 && err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	if err != nil {
		return err
	}
	if len(output) > 0 {
		return fmt.Errorf("%s", string(output))
	}

	return nil
}

type naiveLayerIDMapUpdater struct {
	ProtoDriver
}

// NewNaiveLayerIDMapUpdater wraps the ProtoDriver in a LayerIDMapUpdater that
// uses ChownPathByMaps to update the ownerships in a layer's filesystem tree.
func NewNaiveLayerIDMapUpdater(driver ProtoDriver) LayerIDMapUpdater {
	return &naiveLayerIDMapUpdater{ProtoDriver: driver}
}

// UpdateLayerIDMap walks the layer's filesystem tree, changing the ownership
// information using the toContainer and toHost mappings, using them to replace
// on-disk owner UIDs and GIDs which are "host" values in the first map with
// UIDs and GIDs for "host" values from the second map which correspond to the
// same "container" IDs.
func (n *naiveLayerIDMapUpdater) UpdateLayerIDMap(id string, toContainer, toHost *idtools.IDMappings, mountLabel string) error {
	driver := n.ProtoDriver
	layerFs, err := driver.Get(id, mountLabel)
	if err != nil {
		return err
	}
	defer func() {
		driver.Put(id)
	}()

	return ChownPathByMaps(layerFs, toContainer, toHost)
}
