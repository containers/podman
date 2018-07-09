package buildah

import (
	"archive/tar"
	"io"
	"os"
	"sync"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/pkg/sysregistries"
	"github.com/containers/image/types"
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
func (b *Builder) copyFileWithTar(chownOpts *idtools.IDPair, hasher io.Writer) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			contentReader, contentWriter, err := os.Pipe()
			if err != nil {
				return err
			}
			defer contentReader.Close()
			defer contentWriter.Close()
			var hashError error
			var hashWorker sync.WaitGroup
			hashWorker.Add(1)
			go func() {
				t := tar.NewReader(contentReader)
				_, err := t.Next()
				if err != nil {
					hashError = err
				}
				if _, err = io.Copy(hasher, t); err != nil && err != io.EOF {
					hashError = err
				}
				hashWorker.Done()
			}()
			err = originalUntar(io.TeeReader(tarArchive, contentWriter), dest, options)
			hashWorker.Wait()
			if err == nil {
				err = hashError
			}
			return err
		}
	}
	return archiver.CopyFileWithTar
}

// copyWithTar returns a function which copies a directory tree from outside of
// any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func (b *Builder) copyWithTar(chownOpts *idtools.IDPair, hasher io.Writer) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
	return archiver.CopyWithTar
}

// untarPath returns a function which extracts an archive in a specified
// location into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func (b *Builder) untarPath(chownOpts *idtools.IDPair, hasher io.Writer) func(src, dest string) error {
	convertedUIDMap, convertedGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)
	untarMappings := idtools.NewIDMappingsFromMaps(convertedUIDMap, convertedGIDMap)
	archiver := chrootarchive.NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
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

// getRegistries obtains the list of registries defined in the global registries file.
func getRegistries(sc *types.SystemContext) ([]string, error) {
	searchRegistries, err := sysregistries.GetRegistries(sc)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse the registries.conf file")
	}
	return searchRegistries, nil
}

// hasRegistry returns a bool/err response if the image has a registry in its
// name
func hasRegistry(imageName string) (bool, error) {
	imgRef, err := reference.Parse(imageName)
	if err != nil {
		return false, err
	}
	registry := reference.Domain(imgRef.(reference.Named))
	if registry != "" {
		return true, nil
	}
	return false, nil
}
