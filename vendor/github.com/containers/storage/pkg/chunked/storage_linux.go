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
	"sort"
	"strings"
	"syscall"
	"time"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	driversCopy "github.com/containers/storage/drivers/copy"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/types"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vbatts/tar-split/archive/tar"
	"golang.org/x/sys/unix"
)

const (
	maxNumberMissingChunks  = 1024
	newFileFlags            = (unix.O_CREAT | unix.O_TRUNC | unix.O_WRONLY | unix.O_EXCL)
	containersOverrideXattr = "user.containers.override_stat"
	bigDataKey              = "zstd-chunked-manifest"
)

type chunkedZstdDiffer struct {
	stream         ImageSourceSeekable
	manifest       []byte
	layersMetadata map[string][]internal.ZstdFileMetadata
	layersTarget   map[string]string
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

func copyFileContent(src, destFile, root string, dirfd int, missingDirsMode, mode os.FileMode) (*os.File, int64, error) {
	st, err := os.Stat(src)
	if err != nil {
		return nil, -1, err
	}

	copyWithFileRange, copyWithFileClone := true, true

	// If the destination file already exists, we shouldn't blow it away
	dstFile, err := openFileUnderRoot(destFile, root, dirfd, newFileFlags, mode)
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

func prepareOtherLayersCache(layersMetadata map[string][]internal.ZstdFileMetadata) map[string]map[string]*internal.ZstdFileMetadata {
	maps := make(map[string]map[string]*internal.ZstdFileMetadata)

	for layerID, v := range layersMetadata {
		r := make(map[string]*internal.ZstdFileMetadata)
		for i := range v {
			r[v[i].Digest] = &v[i]
		}
		maps[layerID] = r
	}
	return maps
}

func getLayersCache(store storage.Store) (map[string][]internal.ZstdFileMetadata, map[string]string, error) {
	allLayers, err := store.Layers()
	if err != nil {
		return nil, nil, err
	}

	layersMetadata := make(map[string][]internal.ZstdFileMetadata)
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
		var toc internal.ZstdTOC
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
	return nil, errors.New("blob type not supported for partial retrieval")
}

func makeZstdChunkedDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (*chunkedZstdDiffer, error) {
	manifest, err := readZstdChunkedManifest(iss, blobSize, annotations)
	if err != nil {
		return nil, err
	}
	layersMetadata, layersTarget, err := getLayersCache(store)
	if err != nil {
		return nil, err
	}

	return &chunkedZstdDiffer{
		stream:         iss,
		manifest:       manifest,
		layersMetadata: layersMetadata,
		layersTarget:   layersTarget,
	}, nil
}

func findFileInOtherLayers(file internal.ZstdFileMetadata, root string, dirfd int, layersMetadata map[string]map[string]*internal.ZstdFileMetadata, layersTarget map[string]string, missingDirsMode os.FileMode) (*os.File, int64, error) {
	// this is ugly, needs to be indexed
	for layerID, checksums := range layersMetadata {
		m, found := checksums[file.Digest]
		if !found {
			continue
		}

		source, ok := layersTarget[layerID]
		if !ok {
			continue
		}

		srcDirfd, err := unix.Open(source, unix.O_RDONLY, 0)
		if err != nil {
			continue
		}
		defer unix.Close(srcDirfd)

		srcFile, err := openFileUnderRoot(m.Name, source, srcDirfd, unix.O_RDONLY, 0)
		if err != nil {
			continue
		}
		defer srcFile.Close()

		srcPath := fmt.Sprintf("/proc/self/fd/%d", srcFile.Fd())

		dstFile, written, err := copyFileContent(srcPath, file.Name, root, dirfd, missingDirsMode, 0)
		if err != nil {
			continue
		}
		return dstFile, written, nil
	}
	return nil, 0, nil
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
func findFileOnTheHost(file internal.ZstdFileMetadata, root string, dirfd int, missingDirsMode os.FileMode) (*os.File, int64, error) {
	sourceFile := filepath.Clean(filepath.Join("/", file.Name))
	if !strings.HasPrefix(sourceFile, "/usr/") {
		// limit host deduplication to files under /usr.
		return nil, 0, nil
	}

	st, err := os.Stat(sourceFile)
	if err != nil || !st.Mode().IsRegular() {
		return nil, 0, nil
	}

	if st.Size() != file.Size {
		return nil, 0, nil
	}

	fd, err := unix.Open(sourceFile, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, 0, nil
	}

	f := os.NewFile(uintptr(fd), "fd")
	defer f.Close()

	manifestChecksum, err := digest.Parse(file.Digest)
	if err != nil {
		return nil, 0, err
	}

	checksum, err := getFileDigest(f)
	if err != nil {
		return nil, 0, err
	}

	if checksum != manifestChecksum {
		return nil, 0, nil
	}

	dstFile, written, err := copyFileContent(fmt.Sprintf("/proc/self/fd/%d", fd), file.Name, root, dirfd, missingDirsMode, 0)
	if err != nil {
		return nil, 0, nil
	}

	// calculate the checksum again to make sure the file wasn't modified while it was copied
	if _, err := f.Seek(0, 0); err != nil {
		return nil, 0, err
	}
	checksum, err = getFileDigest(f)
	if err != nil {
		return nil, 0, err
	}
	if checksum != manifestChecksum {
		return nil, 0, nil
	}
	return dstFile, written, nil
}

func maybeDoIDRemap(manifest []internal.ZstdFileMetadata, options *archive.TarOptions) error {
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
	File *internal.ZstdFileMetadata
	Gap  int64
}

func (m missingFile) Length() int64 {
	return m.File.EndOffset - m.File.Offset
}

type missingChunk struct {
	RawChunk ImageSourceChunk
	Files    []missingFile
}

func setFileAttrs(file *os.File, mode os.FileMode, metadata *internal.ZstdFileMetadata, options *archive.TarOptions) error {
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

func openFileUnderRoot(name, root string, dirfd int, flags uint64, mode os.FileMode) (*os.File, error) {
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

func createFileFromZstdStream(dest string, dirfd int, reader io.Reader, missingDirsMode, mode os.FileMode, metadata *internal.ZstdFileMetadata, options *archive.TarOptions) (err error) {
	file, err := openFileUnderRoot(metadata.Name, dest, dirfd, newFileFlags, 0)
	if err != nil {
		return err
	}
	defer func() {
		err2 := file.Close()
		if err == nil {
			err = err2
		}
	}()

	z, err := zstd.NewReader(reader)
	if err != nil {
		return err
	}
	defer z.Close()

	digester := digest.Canonical.Digester()
	checksum := digester.Hash()
	_, err = z.WriteTo(io.MultiWriter(file, checksum))
	if err != nil {
		return err
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

func storeMissingFiles(streams chan io.ReadCloser, errs chan error, dest string, dirfd int, missingChunks []missingChunk, missingDirsMode os.FileMode, options *archive.TarOptions) error {
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
			return errors.Errorf("invalid stream returned %d %d", mc, len(missingChunks))
		}
		if mc == len(missingChunks) {
			return errors.Errorf("too many chunks returned")
		}

		for _, mf := range missingChunks[mc].Files {
			if mf.Gap > 0 {
				limitReader := io.LimitReader(part, mf.Gap)
				_, err := io.Copy(ioutil.Discard, limitReader)
				if err != nil {
					return err
				}
				continue
			}

			limitReader := io.LimitReader(part, mf.Length())

			if err := createFileFromZstdStream(dest, dirfd, limitReader, missingDirsMode, os.FileMode(mf.File.Mode), mf.File, options); err != nil {
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
			gapFile := missingFile{
				Gap: int64(gap),
			}
			prev.RawChunk.Length += uint64(gap) + missingChunks[i].RawChunk.Length
			prev.Files = append(append(prev.Files, gapFile), missingChunks[i].Files...)
		}
	}

	return newMissingChunks
}

func retrieveMissingFiles(input *chunkedZstdDiffer, dest string, dirfd int, missingChunks []missingChunk, missingDirsMode os.FileMode, options *archive.TarOptions) error {
	var chunksToRequest []ImageSourceChunk
	for _, c := range missingChunks {
		chunksToRequest = append(chunksToRequest, c.RawChunk)
	}

	// There are some missing files.  Prepare a multirange request for the missing chunks.
	var streams chan io.ReadCloser
	var err error
	var errs chan error
	for {
		streams, errs, err = input.stream.GetBlobAt(chunksToRequest)
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

	if err := storeMissingFiles(streams, errs, dest, dirfd, missingChunks, missingDirsMode, options); err != nil {
		return err
	}
	return nil
}

func safeMkdir(target string, dirfd int, mode os.FileMode, metadata *internal.ZstdFileMetadata, options *archive.TarOptions) error {
	parent := filepath.Dir(metadata.Name)
	base := filepath.Base(metadata.Name)

	parentFd := dirfd
	if parent != "." {
		parentFile, err := openFileUnderRoot(parent, target, dirfd, unix.O_DIRECTORY|unix.O_PATH|unix.O_RDONLY, 0)
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

	file, err := openFileUnderRoot(metadata.Name, target, dirfd, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	return setFileAttrs(file, mode, metadata, options)
}

func safeLink(target string, dirfd int, mode os.FileMode, metadata *internal.ZstdFileMetadata, options *archive.TarOptions) error {
	sourceFile, err := openFileUnderRoot(metadata.Linkname, target, dirfd, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destDir, destBase := filepath.Dir(metadata.Name), filepath.Base(metadata.Name)
	destDirFd := dirfd
	if destDir != "." {
		f, err := openFileUnderRoot(destDir, target, dirfd, unix.O_RDONLY, 0)
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

	newFile, err := openFileUnderRoot(metadata.Name, target, dirfd, unix.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer newFile.Close()

	return setFileAttrs(newFile, mode, metadata, options)
}

func safeSymlink(target string, dirfd int, mode os.FileMode, metadata *internal.ZstdFileMetadata, options *archive.TarOptions) error {
	destDir, destBase := filepath.Dir(metadata.Name), filepath.Base(metadata.Name)
	destDirFd := dirfd
	if destDir != "." {
		f, err := openFileUnderRoot(destDir, target, dirfd, unix.O_RDONLY, 0)
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
	file, err := openFileUnderRoot(path, d.Root, d.Dirfd, unix.O_RDONLY, 0)
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
		dir, err := openFileUnderRoot(dir, d.Root, d.Dirfd, unix.O_RDONLY, 0)
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
	file, err := openFileUnderRoot(path, d.Root, d.Dirfd, unix.O_PATH, 0)
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
	metadata *internal.ZstdFileMetadata
}

func (d *chunkedZstdDiffer) ApplyDiff(dest string, options *archive.TarOptions) (graphdriver.DriverWithDifferOutput, error) {
	bigData := map[string][]byte{
		bigDataKey: d.manifest,
	}
	output := graphdriver.DriverWithDifferOutput{
		Differ:  d,
		BigData: bigData,
	}

	storeOpts, err := types.DefaultStoreOptionsAutoDetectUID()
	if err != nil {
		return output, err
	}

	enableHostDedup := false
	if value := storeOpts.PullOptions["enable_host_deduplication"]; strings.ToLower(value) == "true" {
		enableHostDedup = true
	}

	// Generate the manifest
	var toc internal.ZstdTOC
	if err := json.Unmarshal(d.manifest, &toc); err != nil {
		return output, err
	}

	whiteoutConverter := archive.GetWhiteoutConverter(options.WhiteoutFormat, options.WhiteoutData)

	var missingChunks []missingChunk
	var mergedEntries []internal.ZstdFileMetadata

	if err := maybeDoIDRemap(toc.Entries, options); err != nil {
		return output, err
	}

	for _, e := range toc.Entries {
		if e.Type == TypeChunk {
			l := len(mergedEntries)
			if l == 0 || mergedEntries[l-1].Type != TypeReg {
				return output, errors.New("chunk type without a regular file")
			}
			mergedEntries[l-1].EndOffset = e.EndOffset
			continue
		}
		mergedEntries = append(mergedEntries, e)
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

	otherLayersCache := prepareOtherLayersCache(d.layersMetadata)

	missingDirsMode := os.FileMode(0700)
	if options.ForceMask != nil {
		missingDirsMode = *options.ForceMask
	}

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
					file, err := openFileUnderRoot(r.Name, dest, dirfd, newFileFlags, 0)
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
			if err := safeMkdir(dest, dirfd, mode, &r, options); err != nil {
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
			if err := safeSymlink(dest, dirfd, mode, &r, options); err != nil {
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

		dstFile, _, err := findFileInOtherLayers(r, dest, dirfd, otherLayersCache, d.layersTarget, missingDirsMode)
		if err != nil {
			return output, err
		}
		if dstFile != nil {
			if err := setFileAttrs(dstFile, mode, &r, options); err != nil {
				dstFile.Close()
				return output, err
			}
			dstFile.Close()
			continue
		}

		if enableHostDedup {
			dstFile, _, err = findFileOnTheHost(r, dest, dirfd, missingDirsMode)
			if err != nil {
				return output, err
			}
			if dstFile != nil {
				if err := setFileAttrs(dstFile, mode, &r, options); err != nil {
					dstFile.Close()
					return output, err
				}
				dstFile.Close()
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
				File: &toc.Entries[i],
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
		if err := retrieveMissingFiles(d, dest, dirfd, missingChunks, missingDirsMode, options); err != nil {
			return output, err
		}
	}

	for _, m := range hardLinks {
		if err := safeLink(m.dest, m.dirfd, m.mode, m.metadata, options); err != nil {
			return output, err
		}
	}

	if totalChunksSize > 0 {
		logrus.Debugf("Missing %d bytes out of %d (%.2f %%)", missingChunksSize, totalChunksSize, float32(missingChunksSize*100.0)/float32(totalChunksSize))
	}
	return output, nil
}
