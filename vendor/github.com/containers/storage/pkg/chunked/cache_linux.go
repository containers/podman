package chunked

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/containers/storage/pkg/ioutils"
	jsoniter "github.com/json-iterator/go"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	cacheKey     = "chunked-manifest-cache"
	cacheVersion = 2

	digestSha256Empty = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type cacheFile struct {
	tagLen    int
	digestLen int
	tags      []byte
	vdata     []byte
}

type layer struct {
	id        string
	cacheFile *cacheFile
	target    string
	// mmapBuffer is nil when the cache file is fully loaded in memory.
	// Otherwise it points to a mmap'ed buffer that is referenced by cacheFile.vdata.
	mmapBuffer []byte

	// reloadWithMmap is set when the current process generates the cache file,
	// and cacheFile reuses the memory buffer used by the generation function.
	// Next time the layer cache is used, attempt to reload the file using
	// mmap.
	reloadWithMmap bool
}

type layersCache struct {
	layers  []*layer
	refs    int
	store   storage.Store
	mutex   sync.RWMutex
	created time.Time
}

var (
	cacheMutex sync.Mutex
	cache      *layersCache
)

func (c *layer) release() {
	runtime.SetFinalizer(c, nil)
	if c.mmapBuffer != nil {
		unix.Munmap(c.mmapBuffer)
	}
}

func layerFinalizer(c *layer) {
	c.release()
}

func (c *layersCache) release() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	c.refs--
	if c.refs != 0 {
		return
	}
	for _, l := range c.layers {
		l.release()
	}
	cache = nil
}

func getLayersCacheRef(store storage.Store) *layersCache {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if cache != nil && cache.store == store && time.Since(cache.created).Minutes() < 10 {
		cache.refs++
		return cache
	}
	cache := &layersCache{
		store:   store,
		refs:    1,
		created: time.Now(),
	}
	return cache
}

func getLayersCache(store storage.Store) (*layersCache, error) {
	c := getLayersCacheRef(store)

	if err := c.load(); err != nil {
		c.release()
		return nil, err
	}
	return c, nil
}

// loadLayerBigData attempts to load the specified cacheKey from a file and mmap its content.
// If the cache is not backed by a file, then it loads the entire content in memory.
// Returns the cache content, and if mmap'ed, the mmap buffer to Munmap.
func (c *layersCache) loadLayerBigData(layerID, bigDataKey string) ([]byte, []byte, error) {
	inputFile, err := c.store.LayerBigData(layerID, bigDataKey)
	if err != nil {
		return nil, nil, err
	}
	defer inputFile.Close()

	// if the cache is backed by a file, attempt to mmap it.
	if osFile, ok := inputFile.(*os.File); ok {
		st, err := osFile.Stat()
		if err != nil {
			logrus.Warningf("Error stat'ing cache file for layer %q: %v", layerID, err)
			goto fallback
		}
		size := st.Size()
		if size == 0 {
			logrus.Warningf("Cache file size is zero for layer %q: %v", layerID, err)
			goto fallback
		}
		buf, err := unix.Mmap(int(osFile.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			logrus.Warningf("Error mmap'ing cache file for layer %q: %v", layerID, err)
			goto fallback
		}
		// best effort advise to the kernel.
		_ = unix.Madvise(buf, unix.MADV_RANDOM)

		return buf, buf, nil
	}
fallback:
	buf, err := io.ReadAll(inputFile)
	return buf, nil, err
}

func (c *layersCache) loadLayerCache(layerID string) (_ *layer, errRet error) {
	buffer, mmapBuffer, err := c.loadLayerBigData(layerID, cacheKey)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// there is no existing cache to load
	if err != nil || buffer == nil {
		return nil, nil
	}
	defer func() {
		if errRet != nil && mmapBuffer != nil {
			unix.Munmap(mmapBuffer)
		}
	}()
	cacheFile, err := readCacheFileFromMemory(buffer)
	if err != nil {
		return nil, err
	}
	return c.createLayer(layerID, cacheFile, mmapBuffer)
}

func (c *layersCache) createCacheFileFromTOC(layerID string) (*layer, error) {
	clFile, err := c.store.LayerBigData(layerID, chunkedLayerDataKey)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var lcd chunkedLayerData
	if err == nil && clFile != nil {
		defer clFile.Close()
		cl, err := io.ReadAll(clFile)
		if err != nil {
			return nil, fmt.Errorf("open manifest file: %w", err)
		}
		json := jsoniter.ConfigCompatibleWithStandardLibrary

		if err := json.Unmarshal(cl, &lcd); err != nil {
			return nil, err
		}
	}
	manifestReader, err := c.store.LayerBigData(layerID, bigDataKey)
	if err != nil {
		return nil, err
	}
	defer manifestReader.Close()

	manifest, err := io.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	cacheFile, err := writeCache(manifest, lcd.Format, layerID, c.store)
	if err != nil {
		return nil, err
	}
	l, err := c.createLayer(layerID, cacheFile, nil)
	if err != nil {
		return nil, err
	}
	l.reloadWithMmap = true
	return l, nil
}

func (c *layersCache) load() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	loadedLayers := make(map[string]*layer)
	for _, r := range c.layers {
		loadedLayers[r.id] = r
	}
	allLayers, err := c.store.Layers()
	if err != nil {
		return err
	}

	var newLayers []*layer
	for _, r := range allLayers {
		// The layer is present in the store and it is already loaded.  Attempt to
		// re-use it if mmap'ed.
		if l, found := loadedLayers[r.ID]; found {
			// If the layer is not marked for re-load, move it to newLayers.
			if !l.reloadWithMmap {
				delete(loadedLayers, r.ID)
				newLayers = append(newLayers, l)
				continue
			}
		}
		// try to read the existing cache file.
		l, err := c.loadLayerCache(r.ID)
		if err != nil {
			logrus.Warningf("Error loading cache file for layer %q: %v", r.ID, err)
		}
		if l != nil {
			newLayers = append(newLayers, l)
			continue
		}
		// the cache file is either not present or broken.  Try to generate it from the TOC.
		l, err = c.createCacheFileFromTOC(r.ID)
		if err != nil {
			logrus.Warningf("Error creating cache file for layer %q: %v", r.ID, err)
		}
		if l != nil {
			newLayers = append(newLayers, l)
		}
	}
	// The layers that are still in loadedLayers are either stale or fully loaded in memory.  Clean them up.
	for _, l := range loadedLayers {
		l.release()
	}
	c.layers = newLayers
	return nil
}

// calculateHardLinkFingerprint calculates a hash that can be used to verify if a file
// is usable for deduplication with hardlinks.
// To calculate the digest, it uses the file payload digest, UID, GID, mode and xattrs.
func calculateHardLinkFingerprint(f *fileMetadata) (string, error) {
	digester := digest.Canonical.Digester()

	modeString := fmt.Sprintf("%d:%d:%o", f.UID, f.GID, f.Mode)
	hash := digester.Hash()

	if _, err := hash.Write([]byte(f.Digest)); err != nil {
		return "", err
	}

	if _, err := hash.Write([]byte(modeString)); err != nil {
		return "", err
	}

	if len(f.Xattrs) > 0 {
		keys := make([]string, 0, len(f.Xattrs))
		for k := range f.Xattrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			if _, err := hash.Write([]byte(k)); err != nil {
				return "", err
			}
			if _, err := hash.Write([]byte(f.Xattrs[k])); err != nil {
				return "", err
			}
		}
	}
	return string(digester.Digest()), nil
}

// generateFileLocation generates a file location in the form $OFFSET:$LEN:$PATH
func generateFileLocation(path string, offset, len uint64) []byte {
	return []byte(fmt.Sprintf("%d:%d:%s", offset, len, path))
}

// generateTag generates a tag in the form $DIGEST$OFFSET@LEN.
// the [OFFSET; LEN] points to the variable length data where the file locations
// are stored.  $DIGEST has length digestLen stored in the cache file file header.
func generateTag(digest string, offset, len uint64) string {
	return fmt.Sprintf("%s%.20d@%.20d", digest, offset, len)
}

type setBigData interface {
	// SetLayerBigData stores a (possibly large) chunk of named data
	SetLayerBigData(id, key string, data io.Reader) error
}

// writeCache write a cache for the layer ID.
// It generates a sorted list of digests with their offset to the path location and offset.
// The same cache is used to lookup files, chunks and candidates for deduplication with hard links.
// There are 3 kind of digests stored:
// - digest(file.payload))
// - digest(digest(file.payload) + file.UID + file.GID + file.mode + file.xattrs)
// - digest(i) for each i in chunks(file payload)
func writeCache(manifest []byte, format graphdriver.DifferOutputFormat, id string, dest setBigData) (*cacheFile, error) {
	var vdata bytes.Buffer
	tagLen := 0
	digestLen := 0
	var tagsBuffer bytes.Buffer

	toc, err := prepareCacheFile(manifest, format)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, k := range toc {
		if k.Digest != "" {
			location := generateFileLocation(k.Name, 0, uint64(k.Size))

			off := uint64(vdata.Len())
			l := uint64(len(location))

			d := generateTag(k.Digest, off, l)
			if tagLen == 0 {
				tagLen = len(d)
			}
			if tagLen != len(d) {
				return nil, errors.New("digest with different length found")
			}
			tags = append(tags, d)

			fp, err := calculateHardLinkFingerprint(k)
			if err != nil {
				return nil, err
			}
			d = generateTag(fp, off, l)
			if tagLen != len(d) {
				return nil, errors.New("digest with different length found")
			}
			tags = append(tags, d)

			if _, err := vdata.Write(location); err != nil {
				return nil, err
			}
			digestLen = len(k.Digest)
		}
		if k.ChunkDigest != "" {
			location := generateFileLocation(k.Name, uint64(k.ChunkOffset), uint64(k.ChunkSize))
			off := uint64(vdata.Len())
			l := uint64(len(location))
			d := generateTag(k.ChunkDigest, off, l)
			if tagLen == 0 {
				tagLen = len(d)
			}
			if tagLen != len(d) {
				return nil, errors.New("digest with different length found")
			}
			tags = append(tags, d)

			if _, err := vdata.Write(location); err != nil {
				return nil, err
			}
			digestLen = len(k.ChunkDigest)
		}
	}

	sort.Strings(tags)

	for _, t := range tags {
		if _, err := tagsBuffer.Write([]byte(t)); err != nil {
			return nil, err
		}
	}

	pipeReader, pipeWriter := io.Pipe()
	errChan := make(chan error, 1)
	go func() {
		defer pipeWriter.Close()
		defer close(errChan)

		// version
		if err := binary.Write(pipeWriter, binary.LittleEndian, uint64(cacheVersion)); err != nil {
			errChan <- err
			return
		}

		// len of a tag
		if err := binary.Write(pipeWriter, binary.LittleEndian, uint64(tagLen)); err != nil {
			errChan <- err
			return
		}

		// len of a digest
		if err := binary.Write(pipeWriter, binary.LittleEndian, uint64(digestLen)); err != nil {
			errChan <- err
			return
		}

		// tags length
		if err := binary.Write(pipeWriter, binary.LittleEndian, uint64(tagsBuffer.Len())); err != nil {
			errChan <- err
			return
		}

		// vdata length
		if err := binary.Write(pipeWriter, binary.LittleEndian, uint64(vdata.Len())); err != nil {
			errChan <- err
			return
		}

		// tags
		if _, err := pipeWriter.Write(tagsBuffer.Bytes()); err != nil {
			errChan <- err
			return
		}

		// variable length data
		if _, err := pipeWriter.Write(vdata.Bytes()); err != nil {
			errChan <- err
			return
		}

		errChan <- nil
	}()
	defer pipeReader.Close()

	counter := ioutils.NewWriteCounter(io.Discard)

	r := io.TeeReader(pipeReader, counter)

	if err := dest.SetLayerBigData(id, cacheKey, r); err != nil {
		return nil, err
	}

	if err := <-errChan; err != nil {
		return nil, err
	}

	logrus.Debugf("Written lookaside cache for layer %q with length %v", id, counter.Count)

	return &cacheFile{
		digestLen: digestLen,
		tagLen:    tagLen,
		tags:      tagsBuffer.Bytes(),
		vdata:     vdata.Bytes(),
	}, nil
}

func readCacheFileFromMemory(bigDataBuffer []byte) (*cacheFile, error) {
	bigData := bytes.NewReader(bigDataBuffer)

	var version, tagLen, digestLen, tagsLen, vdataLen uint64
	if err := binary.Read(bigData, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if version != cacheVersion {
		return nil, nil //nolint: nilnil
	}
	if err := binary.Read(bigData, binary.LittleEndian, &tagLen); err != nil {
		return nil, err
	}
	if err := binary.Read(bigData, binary.LittleEndian, &digestLen); err != nil {
		return nil, err
	}
	if err := binary.Read(bigData, binary.LittleEndian, &tagsLen); err != nil {
		return nil, err
	}
	if err := binary.Read(bigData, binary.LittleEndian, &vdataLen); err != nil {
		return nil, err
	}

	tags := make([]byte, tagsLen)
	if _, err := bigData.Read(tags); err != nil {
		return nil, err
	}

	// retrieve the unread part of the buffer.
	vdata := bigDataBuffer[len(bigDataBuffer)-bigData.Len():]

	return &cacheFile{
		tagLen:    int(tagLen),
		digestLen: int(digestLen),
		tags:      tags,
		vdata:     vdata,
	}, nil
}

func prepareCacheFile(manifest []byte, format graphdriver.DifferOutputFormat) ([]*fileMetadata, error) {
	toc, err := unmarshalToc(manifest)
	if err != nil {
		// ignore errors here.  They might be caused by a different manifest format.
		logrus.Debugf("could not unmarshal manifest: %v", err)
		return nil, nil //nolint: nilnil
	}

	var entries []fileMetadata
	for i := range toc.Entries {
		entries = append(entries, fileMetadata{
			FileMetadata: toc.Entries[i],
		})
	}

	switch format {
	case graphdriver.DifferOutputFormatDir:
	case graphdriver.DifferOutputFormatFlat:
		entries, err = makeEntriesFlat(entries)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}

	var r []*fileMetadata
	chunkSeen := make(map[string]bool)
	for i := range entries {
		d := entries[i].Digest
		if d != "" {
			r = append(r, &entries[i])
			continue
		}

		// chunks do not use hard link dedup so keeping just one candidate is enough
		cd := toc.Entries[i].ChunkDigest
		if cd != "" && !chunkSeen[cd] {
			r = append(r, &entries[i])
			chunkSeen[cd] = true
		}
	}

	return r, nil
}

func (c *layersCache) createLayer(id string, cacheFile *cacheFile, mmapBuffer []byte) (*layer, error) {
	target, err := c.store.DifferTarget(id)
	if err != nil {
		return nil, fmt.Errorf("get checkout directory layer %q: %w", id, err)
	}
	l := &layer{
		id:         id,
		cacheFile:  cacheFile,
		target:     target,
		mmapBuffer: mmapBuffer,
	}
	if mmapBuffer != nil {
		runtime.SetFinalizer(l, layerFinalizer)
	}
	return l, nil
}

func byteSliceAsString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func findTag(digest string, cacheFile *cacheFile) (string, uint64, uint64) {
	if len(digest) != cacheFile.digestLen {
		return "", 0, 0
	}

	nElements := len(cacheFile.tags) / cacheFile.tagLen

	i := sort.Search(nElements, func(i int) bool {
		d := byteSliceAsString(cacheFile.tags[i*cacheFile.tagLen : i*cacheFile.tagLen+cacheFile.digestLen])
		return strings.Compare(d, digest) >= 0
	})
	if i < nElements {
		d := string(cacheFile.tags[i*cacheFile.tagLen : i*cacheFile.tagLen+len(digest)])
		if digest == d {
			startOff := i*cacheFile.tagLen + cacheFile.digestLen
			parts := strings.Split(string(cacheFile.tags[startOff:(i+1)*cacheFile.tagLen]), "@")

			off, _ := strconv.ParseInt(parts[0], 10, 64)

			len, _ := strconv.ParseInt(parts[1], 10, 64)
			return digest, uint64(off), uint64(len)
		}
	}
	return "", 0, 0
}

func (c *layersCache) findDigestInternal(digest string) (string, string, int64, error) {
	if digest == "" {
		return "", "", -1, nil
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, layer := range c.layers {
		digest, off, tagLen := findTag(digest, layer.cacheFile)
		if digest != "" {
			position := string(layer.cacheFile.vdata[off : off+tagLen])
			parts := strings.SplitN(position, ":", 3)
			if len(parts) != 3 {
				continue
			}
			offFile, _ := strconv.ParseInt(parts[0], 10, 64)
			// parts[1] is the chunk length, currently unused.
			return layer.target, parts[2], offFile, nil
		}
	}

	return "", "", -1, nil
}

// findFileInOtherLayers finds the specified file in other layers.
// file is the file to look for.
func (c *layersCache) findFileInOtherLayers(file *fileMetadata, useHardLinks bool) (string, string, error) {
	digest := file.Digest
	if useHardLinks {
		var err error
		digest, err = calculateHardLinkFingerprint(file)
		if err != nil {
			return "", "", err
		}
	}
	target, name, off, err := c.findDigestInternal(digest)
	if off == 0 {
		return target, name, err
	}
	return "", "", nil
}

func (c *layersCache) findChunkInOtherLayers(chunk *internal.FileMetadata) (string, string, int64, error) {
	return c.findDigestInternal(chunk.ChunkDigest)
}

func unmarshalToc(manifest []byte) (*internal.TOC, error) {
	var toc internal.TOC

	iter := jsoniter.ParseBytes(jsoniter.ConfigFastest, manifest)

	for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
		if strings.ToLower(field) == "version" {
			toc.Version = iter.ReadInt()
			continue
		}
		if strings.ToLower(field) != "entries" {
			iter.Skip()
			continue
		}
		for iter.ReadArray() {
			var m internal.FileMetadata
			for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
				switch strings.ToLower(field) {
				case "type":
					m.Type = iter.ReadString()
				case "name":
					m.Name = iter.ReadString()
				case "linkname":
					m.Linkname = iter.ReadString()
				case "mode":
					m.Mode = iter.ReadInt64()
				case "size":
					m.Size = iter.ReadInt64()
				case "uid":
					m.UID = iter.ReadInt()
				case "gid":
					m.GID = iter.ReadInt()
				case "modtime":
					time, err := time.Parse(time.RFC3339, iter.ReadString())
					if err != nil {
						return nil, err
					}
					m.ModTime = &time
				case "accesstime":
					time, err := time.Parse(time.RFC3339, iter.ReadString())
					if err != nil {
						return nil, err
					}
					m.AccessTime = &time
				case "changetime":
					time, err := time.Parse(time.RFC3339, iter.ReadString())
					if err != nil {
						return nil, err
					}
					m.ChangeTime = &time
				case "devmajor":
					m.Devmajor = iter.ReadInt64()
				case "devminor":
					m.Devminor = iter.ReadInt64()
				case "digest":
					m.Digest = iter.ReadString()
				case "offset":
					m.Offset = iter.ReadInt64()
				case "endoffset":
					m.EndOffset = iter.ReadInt64()
				case "chunksize":
					m.ChunkSize = iter.ReadInt64()
				case "chunkoffset":
					m.ChunkOffset = iter.ReadInt64()
				case "chunkdigest":
					m.ChunkDigest = iter.ReadString()
				case "chunktype":
					m.ChunkType = iter.ReadString()
				case "xattrs":
					m.Xattrs = make(map[string]string)
					for key := iter.ReadObject(); key != ""; key = iter.ReadObject() {
						m.Xattrs[key] = iter.ReadString()
					}
				default:
					iter.Skip()
				}
			}
			if m.Type == TypeReg && m.Size == 0 && m.Digest == "" {
				m.Digest = digestSha256Empty
			}
			toc.Entries = append(toc.Entries, m)
		}
	}

	// validate there is no extra data in the provided input.  This is a security measure to avoid
	// that the digest we calculate for the TOC refers to the entire document.
	if iter.Error != nil && iter.Error != io.EOF {
		return nil, iter.Error
	}
	if iter.WhatIsNext() != jsoniter.InvalidValue || !errors.Is(iter.Error, io.EOF) {
		return nil, fmt.Errorf("unexpected data after manifest")
	}

	return &toc, nil
}
