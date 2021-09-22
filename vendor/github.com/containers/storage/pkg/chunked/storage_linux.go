package chunked

import (
	archivetar "archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	driversCopy "github.com/containers/storage/drivers/copy"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/types"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vbatts/tar-split/archive/tar"
	"golang.org/x/sys/unix"
)

const (
	maxNumberMissingChunks  = 1024
	newFileFlags            = (unix.O_CREAT | unix.O_TRUNC | unix.O_EXCL | unix.O_WRONLY)
	containersOverrideXattr = "user.containers.override_stat"
	bigDataKey              = "zstd-chunked-manifest"

	fileTypeZstdChunked = iota
	fileTypeEstargz     = iota
)

type compressedFileType int

type chunkedDiffer struct {
	stream         ImageSourceSeekable
	manifest       []byte
	layersMetadata map[string][]internal.FileMetadata
	layersTarget   map[string]string
	tocOffset      int64
	fileType       compressedFileType

	gzipReader *pgzip.Reader
}

func timeToTimespec(time time.Time) (ts unix.Timespec) {
	if time.IsZero() {
		// Return UTIME_OMIT special value
		ts.Sec = 0
		ts.Nsec = ((1 << 30) - 2)
		return
	}
	return unix.NsecToTimespec(time.UnixNano())
}

func copyFileContent(srcFd int, destFile string, dirfd int, mode os.FileMode, useHardLinks bool) (*os.File, int64, error) {
	src := fmt.Sprintf("/proc/self/fd/%d", srcFd)
	st, err := os.Stat(src)
	if err != nil {
		return nil, -1, err
	}

	copyWithFileRange, copyWithFileClone := true, true

	if useHardLinks {
		destDirPath := filepath.Dir(destFile)
		destBase := filepath.Base(destFile)
		destDir, err := openFileUnderRoot(destDirPath, dirfd, 0, mode)
		if err == nil {
			defer destDir.Close()

			doLink := func() error {
				// Using unix.AT_EMPTY_PATH requires CAP_DAC_READ_SEARCH while this variant that uses
				// /proc/self/fd doesn't and can be used with rootless.
				srcPath := fmt.Sprintf("/proc/self/fd/%d", srcFd)
				return unix.Linkat(unix.AT_FDCWD, srcPath, int(destDir.Fd()), destBase, unix.AT_SYMLINK_FOLLOW)
			}

			err := doLink()

			// if the destination exists, unlink it first and try again
			if err != nil && os.IsExist(err) {
				unix.Unlinkat(int(destDir.Fd()), destBase, 0)
				err = doLink()
			}
			if err == nil {
				return nil, st.Size(), nil
			}
		}
	}

	// If the destination file already exists, we shouldn't blow it away
	dstFile, err := openFileUnderRoot(destFile, dirfd, newFileFlags, mode)
	if err != nil {
		return nil, -1, err
	}

	err = driversCopy.CopyRegularToFile(src, dstFile, st, &copyWithFileRange, &copyWithFileClone)
	if err != nil {
		dstFile.Close()
		return nil, -1, err
	}
	return dstFile, st.Size(), err
}

func prepareOtherLayersCache(layersMetadata map[string][]internal.FileMetadata) map[string]map[string][]*internal.FileMetadata {
	maps := make(map[string]map[string][]*internal.FileMetadata)

	for layerID, v := range layersMetadata {
		r := make(map[string][]*internal.FileMetadata)
		for i := range v {
			if v[i].Digest != "" {
				r[v[i].Digest] = append(r[v[i].Digest], &v[i])
			}
		}
		maps[layerID] = r
	}
	return maps
}

func getLayersCache(store storage.Store) (map[string][]internal.FileMetadata, map[string]string, error) {
	allLayers, err := store.Layers()
	if err != nil {
		return nil, nil, err
	}

	layersMetadata := make(map[string][]internal.FileMetadata)
	layersTarget := make(map[string]string)
	for _, r := range allLayers {
		manifestReader, err := store.LayerBigData(r.ID, bigDataKey)
		if err != nil {
			continue
		}
		defer manifestReader.Close()
		manifest, err := ioutil.ReadAll(manifestReader)
		if err != nil {
			return nil, nil, err
		}
		var toc internal.TOC
		if err := json.Unmarshal(manifest, &toc); err != nil {
			continue
		}
		layersMetadata[r.ID] = toc.Entries
		target, err := store.DifferTarget(r.ID)
		if err != nil {
			return nil, nil, err
		}
		layersTarget[r.ID] = target
	}

	return layersMetadata, layersTarget, nil
}

// GetDiffer returns a differ than can be used with ApplyDiffWithDiffer.
func GetDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (graphdriver.Differ, error) {
	if _, ok := annotations[internal.ManifestChecksumKey]; ok {
		return makeZstdChunkedDiffer(ctx, store, blobSize, annotations, iss)
	}
	if _, ok := annotations[estargz.TOCJSONDigestAnnotation]; ok {
		return makeEstargzChunkedDiffer(ctx, store, blobSize, annotations, iss)
	}
	return nil, errors.New("blob type not supported for partial retrieval")
}

func makeZstdChunkedDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (*chunkedDiffer, error) {
	manifest, tocOffset, err := readZstdChunkedManifest(iss, blobSize, annotations)
	if err != nil {
		return nil, err
	}
	layersMetadata, layersTarget, err := getLayersCache(store)
	if err != nil {
		return nil, err
	}

	return &chunkedDiffer{
		stream:         iss,
		manifest:       manifest,
		layersMetadata: layersMetadata,
		layersTarget:   layersTarget,
		tocOffset:      tocOffset,
		fileType:       fileTypeZstdChunked,
	}, nil
}

func makeEstargzChunkedDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (*chunkedDiffer, error) {
	manifest, tocOffset, err := readEstargzChunkedManifest(iss, blobSize, annotations)
	if err != nil {
		return nil, err
	}
	layersMetadata, layersTarget, err := getLayersCache(store)
	if err != nil {
		return nil, err
	}

	return &chunkedDiffer{
		stream:         iss,
		manifest:       manifest,
		layersMetadata: layersMetadata,
		layersTarget:   layersTarget,
		tocOffset:      tocOffset,
		fileType:       fileTypeEstargz,
	}, nil
}

// copyFileFromOtherLayer copies a file from another layer
// file is the file to look for.
// source is the path to the source layer checkout.
// otherFile contains the metadata for the file.
// dirfd is an open file descriptor to the destination root directory.
// useHardLinks defines whether the deduplication can be performed using hard links.
func copyFileFromOtherLayer(file *internal.FileMetadata, source string, otherFile *internal.FileMetadata, dirfd int, useHardLinks bool) (bool, *os.File, int64, error) {
	srcDirfd, err := unix.Open(source, unix.O_RDONLY, 0)
	if err != nil {
		return false, nil, 0, err
	}
	defer unix.Close(srcDirfd)

	srcFile, err := openFileUnderRoot(otherFile.Name, srcDirfd, unix.O_RDONLY, 0)
	if err != nil {
		return false, nil, 0, err
	}
	defer srcFile.Close()

	dstFile, written, err := copyFileContent(int(srcFile.Fd()), file.Name, dirfd, 0, useHardLinks)
	if err != nil {
		return false, nil, 0, err
	}
	return true, dstFile, written, err
}

// canDedupMetadataWithHardLink says whether it is possible to deduplicate file with otherFile.
// It checks that the two files have the same UID, GID, file mode and xattrs.
func canDedupMetadataWithHardLink(file *internal.FileMetadata, otherFile *internal.FileMetadata) bool {
	if file.UID != otherFile.UID {
		return false
	}
	if file.GID != otherFile.GID {
		return false
	}
	if file.Mode != otherFile.Mode {
		return false
	}
	if !reflect.DeepEqual(file.Xattrs, otherFile.Xattrs) {
		return false
	}
	return true
}

// canDedupFileWithHardLink checks if the specified file can be deduplicated by an
// open file, given its descriptor and stat data.
func canDedupFileWithHardLink(file *internal.FileMetadata, fd int, s os.FileInfo) bool {
	st, ok := s.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	path := fmt.Sprintf("/proc/self/fd/%d", fd)

	listXattrs, err := system.Llistxattr(path)
	if err != nil {
		return false
	}

	xattrsToIgnore := map[string]interface{}{
		"security.selinux": true,
	}

	xattrs := make(map[string]string)
	for _, x := range listXattrs {
		v, err := system.Lgetxattr(path, x)
		if err != nil {
			return false
		}

		if _, found := xattrsToIgnore[x]; found {
			continue
		}
		xattrs[x] = string(v)
	}
	// fill only the attributes used by canDedupMetadataWithHardLink.
	otherFile := internal.FileMetadata{
		UID:    int(st.Uid),
		GID:    int(st.Gid),
		Mode:   int64(st.Mode),
		Xattrs: xattrs,
	}
	return canDedupMetadataWithHardLink(file, &otherFile)
}

// findFileInOtherLayers finds the specified file in other layers.
// file is the file to look for.
// dirfd is an open file descriptor to the checkout root directory.
// layersMetadata contains the metadata for each layer in the storage.
// layersTarget maps each layer to its checkout on disk.
// useHardLinks defines whether the deduplication can be performed using hard links.
func findFileInOtherLayers(file *internal.FileMetadata, dirfd int, layersMetadata map[string]map[string][]*internal.FileMetadata, layersTarget map[string]string, useHardLinks bool) (bool, *os.File, int64, error) {
	// this is ugly, needs to be indexed
	for layerID, checksums := range layersMetadata {
		source, ok := layersTarget[layerID]
		if !ok {
			continue
		}
		files, found := checksums[file.Digest]
		if !found {
			continue
		}
		for _, candidate := range files {
			// check if it is a valid candidate to dedup file
			if useHardLinks && !canDedupMetadataWithHardLink(file, candidate) {
				continue
			}

			found, dstFile, written, err := copyFileFromOtherLayer(file, source, candidate, dirfd, useHardLinks)
			if found && err == nil {
				return found, dstFile, written, err
			}
		}
	}
	// If hard links deduplication was used and it has failed, try again without hard links.
	if useHardLinks {
		return findFileInOtherLayers(file, dirfd, layersMetadata, layersTarget, false)
	}
	return false, nil, 0, nil
}

func getFileDigest(f *os.File) (digest.Digest, error) {
	digester := digest.Canonical.Digester()
	if _, err := io.Copy(digester.Hash(), f); err != nil {
		return "", err
	}
	return digester.Digest(), nil
}

// findFileOnTheHost checks whether the requested file already exist on the host and copies the file content from there if possible.
// It is currently implemented to look only at the file with the same path.  Ideally it can detect the same content also at different
// paths.
// file is the file to look for.
// dirfd is an open fd to the destination checkout.
// useHardLinks defines whether the deduplication can be performed using hard links.
func findFileOnTheHost(file *internal.FileMetadata, dirfd int, useHardLinks bool) (bool, *os.File, int64, error) {
	sourceFile := filepath.Clean(filepath.Join("/", file.Name))
	if !strings.HasPrefix(sourceFile, "/usr/") {
		// limit host deduplication to files under /usr.
		return false, nil, 0, nil
	}

	st, err := os.Stat(sourceFile)
	if err != nil || !st.Mode().IsRegular() {
		return false, nil, 0, nil
	}

	if st.Size() != file.Size {
		return false, nil, 0, nil
	}

	fd, err := unix.Open(sourceFile, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return false, nil, 0, nil
	}

	f := os.NewFile(uintptr(fd), "fd")
	defer f.Close()

	manifestChecksum, err := digest.Parse(file.Digest)
	if err != nil {
		return false, nil, 0, err
	}

	checksum, err := getFileDigest(f)
	if err != nil {
		return false, nil, 0, err
	}

	if checksum != manifestChecksum {
		return false, nil, 0, nil
	}

	// check if the open file can be deduplicated with hard links
	useHardLinks = useHardLinks && canDedupFileWithHardLink(file, fd, st)

	dstFile, written, err := copyFileContent(fd, file.Name, dirfd, 0, useHardLinks)
	if err != nil {
		return false, nil, 0, nil
	}

	// calculate the checksum again to make sure the file wasn't modified while it was copied
	if _, err := f.Seek(0, 0); err != nil {
		dstFile.Close()
		return false, nil, 0, err
	}
	checksum, err = getFileDigest(f)
	if err != nil {
		dstFile.Close()
		return false, nil, 0, err
	}
	if checksum != manifestChecksum {
		dstFile.Close()
		return false, nil, 0, nil
	}
	return true, dstFile, written, nil
}

func maybeDoIDRemap(manifest []internal.FileMetadata, options *archive.TarOptions) error {
	if options.ChownOpts == nil && len(options.UIDMaps) == 0 || len(options.GIDMaps) == 0 {
		return nil
	}

	idMappings := idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps)

	for i := range manifest {
		if options.ChownOpts != nil {
			manifest[i].UID = options.ChownOpts.UID
			manifest[i].GID = options.ChownOpts.GID
		} else {
			pair := idtools.IDPair{
				UID: manifest[i].UID,
				GID: manifest[i].GID,
			}
			var err error
			manifest[i].UID, manifest[i].GID, err = idMappings.ToContainer(pair)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type missingFile struct {
	File *internal.FileMetadata
	Gap  int64
}

func (m missingFile) Length() int64 {
	return m.File.EndOffset - m.File.Offset
}

type missingChunk struct {
	RawChunk ImageSourceChunk
	Files    []missingFile
}

// setFileAttrs sets the file attributes for file given metadata
func setFileAttrs(file *os.File, mode os.FileMode, metadata *internal.FileMetadata, options *archive.TarOptions) error {
	if file == nil || file.Fd() < 0 {
		return errors.Errorf("invalid file")
	}
	fd := int(file.Fd())

	t, err := typeToTarType(metadata.Type)
	if err != nil {
		return err
	}
	if t == tar.TypeSymlink {
		return nil
	}

	if err := unix.Fchown(fd, metadata.UID, metadata.GID); err != nil {
		if !options.IgnoreChownErrors {
			return err
		}
	}

	for k, v := range metadata.Xattrs {
		data, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return err
		}
		if err := unix.Fsetxattr(fd, k, data, 0); err != nil {
			return err
		}
	}

	ts := []unix.Timespec{timeToTimespec(metadata.AccessTime), timeToTimespec(metadata.ModTime)}
	if err := unix.UtimesNanoAt(fd, "", ts, 0); err != nil && errors.Is(err, unix.ENOSYS) {
		return err
	}

	if err := unix.Fchmod(fd, uint32(mode)); err != nil {
		return err
	}
	return nil
}

// openFileUnderRoot safely opens a file under the specified root directory using openat2
// name is the path to open relative to dirfd.
// dirfd is an open file descriptor to the target checkout directory.
// flags are the flags top pass to the open syscall.
// mode specifies the mode to use for newly created files.
func openFileUnderRoot(name string, dirfd int, flags uint64, mode os.FileMode) (*os.File, error) {
	how := unix.OpenHow{
		Flags:   flags,
		Mode:    uint64(mode & 07777),
		Resolve: unix.RESOLVE_IN_ROOT,
	}

	fd, err := unix.Openat2(dirfd, name, &how)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (c *chunkedDiffer) createFileFromCompressedStream(dest string, dirfd int, reader io.Reader, mode os.FileMode, metadata *internal.FileMetadata, options *archive.TarOptions) (err error) {
	file, err := openFileUnderRoot(metadata.Name, dirfd, newFileFlags, 0)
	if err != nil {
		return err
	}
	defer func() {
		err2 := file.Close()
		if err == nil {
			err = err2
		}
	}()

	digester := digest.Canonical.Digester()
	checksum := digester.Hash()
	to := io.MultiWriter(file, checksum)

	switch c.fileType {
	case fileTypeZstdChunked:
		z, err := zstd.NewReader(reader)
		if err != nil {
			return err
		}
		defer z.Close()

		if _, err := io.Copy(to, io.LimitReader(z, metadata.Size)); err != nil {
			return err
		}
		if _, err := io.Copy(ioutil.Discard, reader); err != nil {
			return err
		}
	case fileTypeEstargz:
		if c.gzipReader == nil {
			r, err := pgzip.NewReader(reader)
			if err != nil {
				return err
			}
			c.gzipReader = r
		} else {
			if err := c.gzipReader.Reset(reader); err != nil {
				return err
			}
		}
		defer c.gzipReader.Close()

		if _, err := io.Copy(to, io.LimitReader(c.gzipReader, metadata.Size)); err != nil {
			return err
		}
		if _, err := io.Copy(ioutil.Discard, reader); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown file type %q", c.fileType)
	}

	manifestChecksum, err := digest.Parse(metadata.Digest)
	if err != nil {
		return err
	}
	if digester.Digest() != manifestChecksum {
		return fmt.Errorf("checksum mismatch for %q", dest)
	}
	return setFileAttrs(file, mode, metadata, options)
}

func (c *chunkedDiffer) storeMissingFiles(streams chan io.ReadCloser, errs chan error, dest string, dirfd int, missingChunks []missingChunk, options *archive.TarOptions) error {
	for mc := 0; ; mc++ {
		var part io.ReadCloser
		select {
		case p := <-streams:
			part = p
		case err := <-errs:
			return err
		}
		if part == nil {
			if mc == len(missingChunks) {
				break
			}
			return errors.Errorf("invalid stream returned")
		}
		if mc == len(missingChunks) {
			part.Close()
			return errors.Errorf("too many chunks returned")
		}

		for _, mf := range missingChunks[mc].Files {
			if mf.Gap > 0 {
				limitReader := io.LimitReader(part, mf.Gap)
				_, err := io.Copy(ioutil.Discard, limitReader)
				if err != nil {
					part.Close()
					return err
				}
				continue
			}

			limitReader := io.LimitReader(part, mf.Length())

			if err := c.createFileFromCompressedStream(dest, dirfd, limitReader, os.FileMode(mf.File.Mode), mf.File, options); err != nil {
				part.Close()
				return err
			}
		}
		part.Close()
	}
	return nil
}

func mergeMissingChunks(missingChunks []missingChunk, target int) []missingChunk {
	if len(missingChunks) <= target {
		return missingChunks
	}

	getGap := func(missingChunks []missingChunk, i int) int {
		prev := missingChunks[i-1].RawChunk.Offset + missingChunks[i-1].RawChunk.Length
		return int(missingChunks[i].RawChunk.Offset - prev)
	}

	// this implementation doesn't account for duplicates, so it could merge
	// more than necessary to reach the specified target.  Since target itself
	// is a heuristic value, it doesn't matter.
	var gaps []int
	for i := 1; i < len(missingChunks); i++ {
		gaps = append(gaps, getGap(missingChunks, i))
	}
	sort.Ints(gaps)

	toShrink := len(missingChunks) - target
	targetValue := gaps[toShrink-1]

	newMissingChunks := missingChunks[0:1]
	for i := 1; i < len(missingChunks); i++ {
		gap := getGap(missingChunks, i)
		if gap > targetValue {
			newMissingChunks = append(newMissingChunks, missingChunks[i])
		} else {
			prev := &newMissingChunks[len(newMissingChunks)-1]
			prev.RawChunk.Length += uint64(gap) + missingChunks[i].RawChunk.Length
			if gap > 0 {
				gapFile := missingFile{
					Gap: int64(gap),
				}
				prev.Files = append(prev.Files, gapFile)
			}
			prev.Files = append(prev.Files, missingChunks[i].Files...)
		}
	}
	return newMissingChunks
}

func (c *chunkedDiffer) retrieveMissingFiles(dest string, dirfd int, missingChunks []missingChunk, options *archive.TarOptions) error {
	var chunksToRequest []ImageSourceChunk
	for _, c := range missingChunks {
		chunksToRequest = append(chunksToRequest, c.RawChunk)
	}

	// There are some missing files.  Prepare a multirange request for the missing chunks.
	var streams chan io.ReadCloser
	var err error
	var errs chan error
	for {
		streams, errs, err = c.stream.GetBlobAt(chunksToRequest)
		if err == nil {
			break
		}

		if _, ok := err.(ErrBadRequest); ok {
			requested := len(missingChunks)
			// If the server cannot handle at least 64 chunks in a single request, just give up.
			if requested < 64 {
				return err
			}

			// Merge more chunks to request
			missingChunks = mergeMissingChunks(missingChunks, requested/2)
			continue
		}
		return err
	}

	if err := c.storeMissingFiles(streams, errs, dest, dirfd, missingChunks, options); err != nil {
		return err
	}
	return nil
}

func safeMkdir(dirfd int, mode os.FileMode, metadata *internal.FileMetadata, options *archive.TarOptions) error {
	parent := filepath.Dir(metadata.Name)
	base := filepath.Base(metadata.Name)

	parentFd := dirfd
	if parent != "." {
		parentFile, err := openFileUnderRoot(parent, dirfd, unix.O_DIRECTORY|unix.O_PATH|unix.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer parentFile.Close()
		parentFd = int(parentFile.Fd())
	}

	if err := unix.Mkdirat(parentFd, base, uint32(mode)); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	file, err := openFileUnderRoot(metadata.Name, dirfd, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	return setFileAttrs(file, mode, metadata, options)
}

func safeLink(dirfd int, mode os.FileMode, metadata *internal.FileMetadata, options *archive.TarOptions) error {
	sourceFile, err := openFileUnderRoot(metadata.Linkname, dirfd, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destDir, destBase := filepath.Dir(metadata.Name), filepath.Base(metadata.Name)
	destDirFd := dirfd
	if destDir != "." {
		f, err := openFileUnderRoot(destDir, dirfd, unix.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		destDirFd = int(f.Fd())
	}

	err = unix.Linkat(int(sourceFile.Fd()), "", destDirFd, destBase, unix.AT_EMPTY_PATH)
	if err != nil {
		return err
	}

	newFile, err := openFileUnderRoot(metadata.Name, dirfd, unix.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer newFile.Close()

	return setFileAttrs(newFile, mode, metadata, options)
}

func safeSymlink(dirfd int, mode os.FileMode, metadata *internal.FileMetadata, options *archive.TarOptions) error {
	destDir, destBase := filepath.Dir(metadata.Name), filepath.Base(metadata.Name)
	destDirFd := dirfd
	if destDir != "." {
		f, err := openFileUnderRoot(destDir, dirfd, unix.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		destDirFd = int(f.Fd())
	}

	return unix.Symlinkat(metadata.Linkname, destDirFd, destBase)
}

type whiteoutHandler struct {
	Dirfd int
	Root  string
}

func (d whiteoutHandler) Setxattr(path, name string, value []byte) error {
	file, err := openFileUnderRoot(path, d.Dirfd, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	return unix.Fsetxattr(int(file.Fd()), name, value, 0)
}

func (d whiteoutHandler) Mknod(path string, mode uint32, dev int) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	dirfd := d.Dirfd
	if dir != "" {
		dir, err := openFileUnderRoot(dir, d.Dirfd, unix.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer dir.Close()

		dirfd = int(dir.Fd())
	}

	return unix.Mknodat(dirfd, base, mode, dev)
}

func checkChownErr(err error, name string, uid, gid int) error {
	if errors.Is(err, syscall.EINVAL) {
		return errors.Wrapf(err, "potentially insufficient UIDs or GIDs available in user namespace (requested %d:%d for %s): Check /etc/subuid and /etc/subgid", uid, gid, name)
	}
	return err
}

func (d whiteoutHandler) Chown(path string, uid, gid int) error {
	file, err := openFileUnderRoot(path, d.Dirfd, unix.O_PATH, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := unix.Fchownat(int(file.Fd()), "", uid, gid, unix.AT_EMPTY_PATH); err != nil {
		var stat unix.Stat_t
		if unix.Fstat(int(file.Fd()), &stat) == nil {
			if stat.Uid == uint32(uid) && stat.Gid == uint32(gid) {
				return nil
			}
		}
		return checkChownErr(err, path, uid, gid)
	}
	return nil
}

type hardLinkToCreate struct {
	dest     string
	dirfd    int
	mode     os.FileMode
	metadata *internal.FileMetadata
}

func parseBooleanPullOption(storeOpts *storage.StoreOptions, name string, def bool) bool {
	if value, ok := storeOpts.PullOptions[name]; ok {
		return strings.ToLower(value) == "true"
	}
	return def
}

func (c *chunkedDiffer) ApplyDiff(dest string, options *archive.TarOptions) (graphdriver.DriverWithDifferOutput, error) {
	bigData := map[string][]byte{
		bigDataKey: c.manifest,
	}
	output := graphdriver.DriverWithDifferOutput{
		Differ:  c,
		BigData: bigData,
	}

	storeOpts, err := types.DefaultStoreOptionsAutoDetectUID()
	if err != nil {
		return output, err
	}

	if !parseBooleanPullOption(&storeOpts, "enable_partial_images", false) {
		return output, errors.New("enable_partial_images not configured")
	}

	enableHostDedup := parseBooleanPullOption(&storeOpts, "enable_host_deduplication", false)

	// When the hard links deduplication is used, file attributes are ignored because setting them
	// modifies the source file as well.
	useHardLinks := parseBooleanPullOption(&storeOpts, "use_hard_links", false)

	// Generate the manifest
	var toc internal.TOC
	if err := json.Unmarshal(c.manifest, &toc); err != nil {
		return output, err
	}

	whiteoutConverter := archive.GetWhiteoutConverter(options.WhiteoutFormat, options.WhiteoutData)

	var missingChunks []missingChunk

	mergedEntries, err := c.mergeTocEntries(c.fileType, toc.Entries)
	if err != nil {
		return output, err
	}
	if err := maybeDoIDRemap(mergedEntries, options); err != nil {
		return output, err
	}

	if options.ForceMask != nil {
		uid, gid, mode, err := archive.GetFileOwner(dest)
		if err == nil {
			value := fmt.Sprintf("%d:%d:0%o", uid, gid, mode)
			if err := unix.Setxattr(dest, containersOverrideXattr, []byte(value), 0); err != nil {
				return output, err
			}
		}
	}

	dirfd, err := unix.Open(dest, unix.O_RDONLY|unix.O_PATH, 0)
	if err != nil {
		return output, err
	}
	defer unix.Close(dirfd)

	otherLayersCache := prepareOtherLayersCache(c.layersMetadata)

	// hardlinks can point to missing files.  So create them after all files
	// are retrieved
	var hardLinks []hardLinkToCreate

	missingChunksSize, totalChunksSize := int64(0), int64(0)
	for i, r := range mergedEntries {
		if options.ForceMask != nil {
			value := fmt.Sprintf("%d:%d:0%o", r.UID, r.GID, r.Mode&07777)
			r.Xattrs[containersOverrideXattr] = base64.StdEncoding.EncodeToString([]byte(value))
			r.Mode = int64(*options.ForceMask)
		}

		mode := os.FileMode(r.Mode)

		r.Name = filepath.Clean(r.Name)
		r.Linkname = filepath.Clean(r.Linkname)

		t, err := typeToTarType(r.Type)
		if err != nil {
			return output, err
		}
		if whiteoutConverter != nil {
			hdr := archivetar.Header{
				Typeflag: t,
				Name:     r.Name,
				Linkname: r.Linkname,
				Size:     r.Size,
				Mode:     r.Mode,
				Uid:      r.UID,
				Gid:      r.GID,
			}
			handler := whiteoutHandler{
				Dirfd: dirfd,
				Root:  dest,
			}
			writeFile, err := whiteoutConverter.ConvertReadWithHandler(&hdr, r.Name, &handler)
			if err != nil {
				return output, err
			}
			if !writeFile {
				continue
			}
		}
		switch t {
		case tar.TypeReg:
			// Create directly empty files.
			if r.Size == 0 {
				// Used to have a scope for cleanup.
				createEmptyFile := func() error {
					file, err := openFileUnderRoot(r.Name, dirfd, newFileFlags, 0)
					if err != nil {
						return err
					}
					defer file.Close()
					if err := setFileAttrs(file, mode, &r, options); err != nil {
						return err
					}
					return nil
				}
				if err := createEmptyFile(); err != nil {
					return output, err
				}
				continue
			}

		case tar.TypeDir:
			if err := safeMkdir(dirfd, mode, &r, options); err != nil {
				return output, err
			}
			continue

		case tar.TypeLink:
			dest := dest
			dirfd := dirfd
			mode := mode
			r := r
			hardLinks = append(hardLinks, hardLinkToCreate{
				dest:     dest,
				dirfd:    dirfd,
				mode:     mode,
				metadata: &r,
			})
			continue

		case tar.TypeSymlink:
			if err := safeSymlink(dirfd, mode, &r, options); err != nil {
				return output, err
			}
			continue

		case tar.TypeChar:
		case tar.TypeBlock:
		case tar.TypeFifo:
			/* Ignore.  */
		default:
			return output, fmt.Errorf("invalid type %q", t)
		}

		totalChunksSize += r.Size

		found, dstFile, _, err := findFileInOtherLayers(&r, dirfd, otherLayersCache, c.layersTarget, useHardLinks)
		if err != nil {
			return output, err
		}
		if dstFile != nil {
			if err := setFileAttrs(dstFile, mode, &r, options); err != nil {
				dstFile.Close()
				return output, err
			}
			dstFile.Close()
		}
		if found {
			continue
		}

		if enableHostDedup {
			found, dstFile, _, err = findFileOnTheHost(&r, dirfd, useHardLinks)
			if err != nil {
				return output, err
			}
			if dstFile != nil {
				if err := setFileAttrs(dstFile, mode, &r, options); err != nil {
					dstFile.Close()
					return output, err
				}
				dstFile.Close()
			}
			if found {
				continue
			}
		}

		missingChunksSize += r.Size
		if t == tar.TypeReg {
			rawChunk := ImageSourceChunk{
				Offset: uint64(r.Offset),
				Length: uint64(r.EndOffset - r.Offset),
			}

			file := missingFile{
				File: &mergedEntries[i],
			}

			missingChunks = append(missingChunks, missingChunk{
				RawChunk: rawChunk,
				Files: []missingFile{
					file,
				},
			})
		}
	}
	// There are some missing files.  Prepare a multirange request for the missing chunks.
	if len(missingChunks) > 0 {
		missingChunks = mergeMissingChunks(missingChunks, maxNumberMissingChunks)
		if err := c.retrieveMissingFiles(dest, dirfd, missingChunks, options); err != nil {
			return output, err
		}
	}

	for _, m := range hardLinks {
		if err := safeLink(m.dirfd, m.mode, m.metadata, options); err != nil {
			return output, err
		}
	}

	if totalChunksSize > 0 {
		logrus.Debugf("Missing %d bytes out of %d (%.2f %%)", missingChunksSize, totalChunksSize, float32(missingChunksSize*100.0)/float32(totalChunksSize))
	}
	return output, nil
}

func (c *chunkedDiffer) mergeTocEntries(fileType compressedFileType, entries []internal.FileMetadata) ([]internal.FileMetadata, error) {
	var mergedEntries []internal.FileMetadata
	var prevEntry *internal.FileMetadata
	for _, entry := range entries {
		e := entry

		// ignore the metadata files for the estargz format.
		if fileType == fileTypeEstargz && (e.Name == estargz.PrefetchLandmark || e.Name == estargz.NoPrefetchLandmark || e.Name == estargz.TOCTarName) {
			continue
		}

		if e.Type == TypeChunk {
			if prevEntry == nil || prevEntry.Type != TypeReg {
				return nil, errors.New("chunk type without a regular file")
			}
			prevEntry.EndOffset = e.EndOffset
			continue
		}
		mergedEntries = append(mergedEntries, e)
		prevEntry = &e
	}
	// stargz/estargz doesn't store EndOffset so let's calculate it here
	lastOffset := c.tocOffset
	for i := len(mergedEntries) - 1; i >= 0; i-- {
		if mergedEntries[i].EndOffset == 0 {
			mergedEntries[i].EndOffset = lastOffset
		}
		if mergedEntries[i].Offset != 0 {
			lastOffset = mergedEntries[i].Offset
		}
	}
	return mergedEntries, nil
}
