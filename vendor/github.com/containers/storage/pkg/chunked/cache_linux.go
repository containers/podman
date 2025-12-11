package chunked

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
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
)

const (
	cacheKey     = "chunked-manifest-cache"
	cacheVersion = 1

	digestSha256Empty = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type metadata struct {
	tagLen    int
	digestLen int
	tags      []byte
	vdata     []byte
}

type layer struct {
	id       string
	metadata *metadata
	target   string
}

type layersCache struct {
	layers  []layer
	refs    int
	store   storage.Store
	mutex   sync.RWMutex
	created time.Time
}

var (
	cacheMutex sync.Mutex
	cache      *layersCache
)

func (c *layersCache) release() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	c.refs--
	if c.refs == 0 {
		cache = nil
	}
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

func (c *layersCache) load() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	allLayers, err := c.store.Layers()
	if err != nil {
		return err
	}
	existingLayers := make(map[string]string)
	for _, r := range c.layers {
		existingLayers[r.id] = r.target
	}

	currentLayers := make(map[string]string)
	for _, r := range allLayers {
		currentLayers[r.ID] = r.ID
		if _, found := existingLayers[r.ID]; found {
			continue
		}

		bigData, err := c.store.LayerBigData(r.ID, cacheKey)
		// if the cache already exists, read and use it
		if err == nil {
			defer bigData.Close()
			metadata, err := readMetadataFromCache(bigData)
			if err == nil {
				c.addLayer(r.ID, metadata)
				continue
			}
			logrus.Warningf("Error reading cache file for layer %q: %v", r.ID, err)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		var lcd chunkedLayerData

		clFile, err := c.store.LayerBigData(r.ID, chunkedLayerDataKey)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if clFile != nil {
			cl, err := io.ReadAll(clFile)
			if err != nil {
				return fmt.Errorf("open manifest file for layer %q: %w", r.ID, err)
			}
			json := jsoniter.ConfigCompatibleWithStandardLibrary
			if err := json.Unmarshal(cl, &lcd); err != nil {
				return err
			}
		}

		// otherwise create it from the layer TOC.
		manifestReader, err := c.store.LayerBigData(r.ID, bigDataKey)
		if err != nil {
			continue
		}
		defer manifestReader.Close()

		manifest, err := io.ReadAll(manifestReader)
		if err != nil {
			return fmt.Errorf("open manifest file for layer %q: %w", r.ID, err)
		}

		metadata, err := writeCache(manifest, lcd.Format, r.ID, c.store)
		if err == nil {
			c.addLayer(r.ID, metadata)
		}
	}

	var newLayers []layer
	for _, l := range c.layers {
		if _, found := currentLayers[l.id]; found {
			newLayers = append(newLayers, l)
		}
	}
	c.layers = newLayers

	return nil
}

// calculateHardLinkFingerprint calculates a hash that can be used to verify if a file
// is usable for deduplication with hardlinks.
// To calculate the digest, it uses the file payload digest, UID, GID, mode and xattrs.
func calculateHardLinkFingerprint(f *internal.FileMetadata) (string, error) {
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

// generateFileLocation generates a file location in the form $OFFSET@$PATH
func generateFileLocation(path string, offset uint64) []byte {
	return []byte(fmt.Sprintf("%d@%s", offset, path))
}

// generateTag generates a tag in the form $DIGEST$OFFSET@LEN.
// the [OFFSET; LEN] points to the variable length data where the file locations
// are stored.  $DIGEST has length digestLen stored in the metadata file header.
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
func writeCache(manifest []byte, format graphdriver.DifferOutputFormat, id string, dest setBigData) (*metadata, error) {
	var vdata bytes.Buffer
	tagLen := 0
	digestLen := 0
	var tagsBuffer bytes.Buffer

	toc, err := prepareMetadata(manifest, format)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, k := range toc {
		if k.Digest != "" {
			location := generateFileLocation(k.Name, 0)

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
			location := generateFileLocation(k.Name, uint64(k.ChunkOffset))
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

	return &metadata{
		digestLen: digestLen,
		tagLen:    tagLen,
		tags:      tagsBuffer.Bytes(),
		vdata:     vdata.Bytes(),
	}, nil
}

func readMetadataFromCache(bigData io.Reader) (*metadata, error) {
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

	vdata := make([]byte, vdataLen)
	if _, err := bigData.Read(vdata); err != nil {
		return nil, err
	}

	return &metadata{
		tagLen:    int(tagLen),
		digestLen: int(digestLen),
		tags:      tags,
		vdata:     vdata,
	}, nil
}

func prepareMetadata(manifest []byte, format graphdriver.DifferOutputFormat) ([]*internal.FileMetadata, error) {
	toc, err := unmarshalToc(manifest)
	if err != nil {
		// ignore errors here.  They might be caused by a different manifest format.
		logrus.Debugf("could not unmarshal manifest: %v", err)
		return nil, nil //nolint: nilnil
	}

	switch format {
	case graphdriver.DifferOutputFormatDir:
	case graphdriver.DifferOutputFormatFlat:
		toc.Entries, err = makeEntriesFlat(toc.Entries)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}

	var r []*internal.FileMetadata
	chunkSeen := make(map[string]bool)
	for i := range toc.Entries {
		d := toc.Entries[i].Digest
		if d != "" {
			r = append(r, &toc.Entries[i])
			continue
		}

		// chunks do not use hard link dedup so keeping just one candidate is enough
		cd := toc.Entries[i].ChunkDigest
		if cd != "" && !chunkSeen[cd] {
			r = append(r, &toc.Entries[i])
			chunkSeen[cd] = true
		}
	}

	return r, nil
}

func (c *layersCache) addLayer(id string, metadata *metadata) error {
	target, err := c.store.DifferTarget(id)
	if err != nil {
		return fmt.Errorf("get checkout directory layer %q: %w", id, err)
	}

	l := layer{
		id:       id,
		metadata: metadata,
		target:   target,
	}
	c.layers = append(c.layers, l)
	return nil
}

func byteSliceAsString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func findTag(digest string, metadata *metadata) (string, uint64, uint64) {
	if len(digest) != metadata.digestLen {
		return "", 0, 0
	}

	nElements := len(metadata.tags) / metadata.tagLen

	i := sort.Search(nElements, func(i int) bool {
		d := byteSliceAsString(metadata.tags[i*metadata.tagLen : i*metadata.tagLen+metadata.digestLen])
		return strings.Compare(d, digest) >= 0
	})
	if i < nElements {
		d := string(metadata.tags[i*metadata.tagLen : i*metadata.tagLen+len(digest)])
		if digest == d {
			startOff := i*metadata.tagLen + metadata.digestLen
			parts := strings.Split(string(metadata.tags[startOff:(i+1)*metadata.tagLen]), "@")
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
		digest, off, len := findTag(digest, layer.metadata)
		if digest != "" {
			position := string(layer.metadata.vdata[off : off+len])
			parts := strings.SplitN(position, "@", 2)
			offFile, _ := strconv.ParseInt(parts[0], 10, 64)
			return layer.target, parts[1], offFile, nil
		}
	}

	return "", "", -1, nil
}

// findFileInOtherLayers finds the specified file in other layers.
// file is the file to look for.
func (c *layersCache) findFileInOtherLayers(file *internal.FileMetadata, useHardLinks bool) (string, string, error) {
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
	var buf bytes.Buffer
	count := 0
	var toc internal.TOC

	iter := jsoniter.ParseBytes(jsoniter.ConfigFastest, manifest)
	for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
		if strings.ToLower(field) != "entries" {
			iter.Skip()
			continue
		}
		for iter.ReadArray() {
			for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
				switch strings.ToLower(field) {
				case "type", "name", "linkname", "digest", "chunkdigest", "chunktype", "modtime", "accesstime", "changetime":
					count += len(iter.ReadStringAsSlice())
				case "xattrs":
					for key := iter.ReadObject(); key != ""; key = iter.ReadObject() {
						count += len(iter.ReadStringAsSlice())
					}
				default:
					iter.Skip()
				}
			}
		}
		break
	}

	buf.Grow(count)

	getString := func(b []byte) string {
		from := buf.Len()
		buf.Write(b)
		to := buf.Len()
		return byteSliceAsString(buf.Bytes()[from:to])
	}

	iter = jsoniter.ParseBytes(jsoniter.ConfigFastest, manifest)
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
					m.Type = getString(iter.ReadStringAsSlice())
				case "name":
					m.Name = getString(iter.ReadStringAsSlice())
				case "linkname":
					m.Linkname = getString(iter.ReadStringAsSlice())
				case "mode":
					m.Mode = iter.ReadInt64()
				case "size":
					m.Size = iter.ReadInt64()
				case "uid":
					m.UID = iter.ReadInt()
				case "gid":
					m.GID = iter.ReadInt()
				case "modtime":
					time, err := time.Parse(time.RFC3339, byteSliceAsString(iter.ReadStringAsSlice()))
					if err != nil {
						return nil, err
					}
					m.ModTime = &time
				case "accesstime":
					time, err := time.Parse(time.RFC3339, byteSliceAsString(iter.ReadStringAsSlice()))
					if err != nil {
						return nil, err
					}
					m.AccessTime = &time
				case "changetime":
					time, err := time.Parse(time.RFC3339, byteSliceAsString(iter.ReadStringAsSlice()))
					if err != nil {
						return nil, err
					}
					m.ChangeTime = &time
				case "devmajor":
					m.Devmajor = iter.ReadInt64()
				case "devminor":
					m.Devminor = iter.ReadInt64()
				case "digest":
					m.Digest = getString(iter.ReadStringAsSlice())
				case "offset":
					m.Offset = iter.ReadInt64()
				case "endoffset":
					m.EndOffset = iter.ReadInt64()
				case "chunksize":
					m.ChunkSize = iter.ReadInt64()
				case "chunkoffset":
					m.ChunkOffset = iter.ReadInt64()
				case "chunkdigest":
					m.ChunkDigest = getString(iter.ReadStringAsSlice())
				case "chunktype":
					m.ChunkType = getString(iter.ReadStringAsSlice())
				case "xattrs":
					m.Xattrs = make(map[string]string)
					for key := iter.ReadObject(); key != ""; key = iter.ReadObject() {
						value := iter.ReadStringAsSlice()
						m.Xattrs[key] = getString(value)
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
		break
	}
	toc.StringsBuf = buf
	return &toc, nil
}
