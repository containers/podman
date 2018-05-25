package buildah

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/reexec"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// InitReexec is a wrapper for reexec.Init().  It should be called at
// the start of main(), and if it returns true, main() should return
// immediately.
func InitReexec() bool {
	return reexec.Init()
}

func copyStringStringMap(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}

func copyStringSlice(s []string) []string {
	t := make([]string, len(s))
	copy(t, s)
	return t
}

func stringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func convertStorageIDMaps(UIDMap, GIDMap []idtools.IDMap) ([]rspec.LinuxIDMapping, []rspec.LinuxIDMapping) {
	uidmap := make([]rspec.LinuxIDMapping, 0, len(UIDMap))
	gidmap := make([]rspec.LinuxIDMapping, 0, len(GIDMap))
	for _, m := range UIDMap {
		uidmap = append(uidmap, rspec.LinuxIDMapping{
			HostID:      uint32(m.HostID),
			ContainerID: uint32(m.ContainerID),
			Size:        uint32(m.Size),
		})
	}
	for _, m := range GIDMap {
		gidmap = append(gidmap, rspec.LinuxIDMapping{
			HostID:      uint32(m.HostID),
			ContainerID: uint32(m.ContainerID),
			Size:        uint32(m.Size),
		})
	}
	return uidmap, gidmap
}

func convertRuntimeIDMaps(UIDMap, GIDMap []rspec.LinuxIDMapping) ([]idtools.IDMap, []idtools.IDMap) {
	uidmap := make([]idtools.IDMap, 0, len(UIDMap))
	gidmap := make([]idtools.IDMap, 0, len(GIDMap))
	for _, m := range UIDMap {
		uidmap = append(uidmap, idtools.IDMap{
			HostID:      int(m.HostID),
			ContainerID: int(m.ContainerID),
			Size:        int(m.Size),
		})
	}
	for _, m := range GIDMap {
		gidmap = append(gidmap, idtools.IDMap{
			HostID:      int(m.HostID),
			ContainerID: int(m.ContainerID),
			Size:        int(m.Size),
		})
	}
	return uidmap, gidmap
}

// copyFileWithTar returns a function which copies a single file from outside
// of any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func (b *Builder) copyFileWithTar(chownOpts *idtools.IDPair) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	return archiver.CopyFileWithTar
}

// copyWithTar returns a function which copies a directory tree from outside of
// any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func (b *Builder) copyWithTar(chownOpts *idtools.IDPair) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	return archiver.CopyWithTar
}

// untarPath returns a function which extracts an archive in a specified
// location into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func (b *Builder) untarPath(chownOpts *idtools.IDPair) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	return archiver.UntarPath
}

// tarPath returns a function which creates an archive of a specified
// location in the container's filesystem, mapping permissions using the
// container's ID maps
func (b *Builder) tarPath() func(path string) (io.ReadCloser, error) {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	tarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	return func(path string) (io.ReadCloser, error) {
		return archive.TarWithOptions(path, &archive.TarOptions{
			Compression: archive.Uncompressed,
			UIDMaps:     tarMappings.UIDs(),
			GIDMaps:     tarMappings.GIDs(),
		})
	}
}

// getProcIDMappings reads mappings from the named node under /proc.
func getProcIDMappings(path string) ([]rspec.LinuxIDMapping, error) {
	var mappings []rspec.LinuxIDMapping
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading ID mappings from %q", path)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, errors.Errorf("line %q from %q has %d fields, not 3", line, path, len(fields))
		}
		cid, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing container ID value %q from line %q in %q", fields[0], line, path)
		}
		hid, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing host ID value %q from line %q in %q", fields[1], line, path)
		}
		size, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing size value %q from line %q in %q", fields[2], line, path)
		}
		mappings = append(mappings, rspec.LinuxIDMapping{ContainerID: uint32(cid), HostID: uint32(hid), Size: uint32(size)})
	}
	return mappings, nil
}

// getHostIDs uses ID mappings to compute the host-level IDs that will
// correspond to a UID/GID pair in the container.
func getHostIDs(uidmap, gidmap []rspec.LinuxIDMapping, uid, gid uint32) (uint32, uint32, error) {
	uidMapped := true
	for _, m := range uidmap {
		uidMapped = false
		if uid >= m.ContainerID && uid < m.ContainerID+m.Size {
			uid = (uid - m.ContainerID) + m.HostID
			uidMapped = true
			break
		}
	}
	if !uidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings, but doesn't map UID %d", uid)
	}
	gidMapped := true
	for _, m := range gidmap {
		gidMapped = false
		if gid >= m.ContainerID && gid < m.ContainerID+m.Size {
			gid = (gid - m.ContainerID) + m.HostID
			gidMapped = true
			break
		}
	}
	if !gidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings, but doesn't map GID %d", gid)
	}
	return uid, gid, nil
}

// getHostRootIDs uses ID mappings in spec to compute the host-level IDs that will
// correspond to UID/GID 0/0 in the container.
func getHostRootIDs(spec *rspec.Spec) (uint32, uint32, error) {
	if spec.Linux == nil {
		return 0, 0, nil
	}
	return getHostIDs(spec.Linux.UIDMappings, spec.Linux.GIDMappings, 0, 0)
}
