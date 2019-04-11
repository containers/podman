package chrootarchive

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	rsystem "github.com/opencontainers/runc/libcontainer/system"
	"github.com/pkg/errors"
)

// NewArchiver returns a new Archiver which uses chrootarchive.Untar
func NewArchiver(idMappings *idtools.IDMappings) *archive.Archiver {
	archiver := archive.NewArchiver(idMappings)
	archiver.Untar = Untar
	return archiver
}

// NewArchiverWithChown returns a new Archiver which uses chrootarchive.Untar and the provided ID mapping configuration on both ends
func NewArchiverWithChown(tarIDMappings *idtools.IDMappings, chownOpts *idtools.IDPair, untarIDMappings *idtools.IDMappings) *archive.Archiver {
	archiver := archive.NewArchiverWithChown(tarIDMappings, chownOpts, untarIDMappings)
	archiver.Untar = Untar
	return archiver
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive may be compressed with one of the following algorithms:
//  identity (uncompressed), gzip, bzip2, xz.
func Untar(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return untarHandler(tarArchive, dest, options, true)
}

// UntarUncompressed reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive must be an uncompressed stream.
func UntarUncompressed(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return untarHandler(tarArchive, dest, options, false)
}

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *archive.TarOptions, decompress bool) error {
	if tarArchive == nil {
		return fmt.Errorf("Empty archive")
	}
	if options == nil {
		options = &archive.TarOptions{}
		options.InUserNS = rsystem.RunningInUserNS()
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	idMappings := idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps)
	rootIDs := idMappings.RootPair()

	dest = filepath.Clean(dest)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := idtools.MkdirAllAndChownNew(dest, 0755, rootIDs); err != nil {
			return err
		}
	}

	r := ioutil.NopCloser(tarArchive)
	if decompress {
		decompressedArchive, err := archive.DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	return invokeUnpack(r, dest, options)
}

// CopyFileWithTarAndChown returns a function which copies a single file from outside
// of any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func CopyFileWithTarAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			contentReader, contentWriter, err := os.Pipe()
			if err != nil {
				return errors.Wrapf(err, "error creating pipe extract data to %q", dest)
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
			if err = originalUntar(io.TeeReader(tarArchive, contentWriter), dest, options); err != nil {
				err = errors.Wrapf(err, "error extracting data to %q while copying", dest)
			}
			hashWorker.Wait()
			if err == nil {
				err = errors.Wrapf(hashError, "error calculating digest of data for %q while copying", dest)
			}
			return err
		}
	}
	return archiver.CopyFileWithTar
}

// CopyWithTarAndChown returns a function which copies a directory tree from outside of
// any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func CopyWithTarAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
	return archiver.CopyWithTar
}

// UntarPathAndChown returns a function which extracts an archive in a specified
// location into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func UntarPathAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
	return archiver.UntarPath
}
