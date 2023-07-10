package internal

// NOTE: This is used from github.com/containers/image by callers that
// don't otherwise use containers/storage, so don't make this depend on any
// larger software like the graph drivers.

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
)

type TOC struct {
	Version int            `json:"version"`
	Entries []FileMetadata `json:"entries"`

	// internal: used by unmarshalToc
	StringsBuf bytes.Buffer `json:"-"`
}

type FileMetadata struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Linkname   string            `json:"linkName,omitempty"`
	Mode       int64             `json:"mode,omitempty"`
	Size       int64             `json:"size,omitempty"`
	UID        int               `json:"uid,omitempty"`
	GID        int               `json:"gid,omitempty"`
	ModTime    *time.Time        `json:"modtime,omitempty"`
	AccessTime *time.Time        `json:"accesstime,omitempty"`
	ChangeTime *time.Time        `json:"changetime,omitempty"`
	Devmajor   int64             `json:"devMajor,omitempty"`
	Devminor   int64             `json:"devMinor,omitempty"`
	Xattrs     map[string]string `json:"xattrs,omitempty"`
	Digest     string            `json:"digest,omitempty"`
	Offset     int64             `json:"offset,omitempty"`
	EndOffset  int64             `json:"endOffset,omitempty"`

	ChunkSize   int64  `json:"chunkSize,omitempty"`
	ChunkOffset int64  `json:"chunkOffset,omitempty"`
	ChunkDigest string `json:"chunkDigest,omitempty"`
	ChunkType   string `json:"chunkType,omitempty"`

	// internal: computed by mergeTOCEntries.
	Chunks []*FileMetadata `json:"-"`
}

const (
	ChunkTypeData  = ""
	ChunkTypeZeros = "zeros"
)

const (
	TypeReg     = "reg"
	TypeChunk   = "chunk"
	TypeLink    = "hardlink"
	TypeChar    = "char"
	TypeBlock   = "block"
	TypeDir     = "dir"
	TypeFifo    = "fifo"
	TypeSymlink = "symlink"
)

var TarTypes = map[byte]string{
	tar.TypeReg:     TypeReg,
	tar.TypeRegA:    TypeReg,
	tar.TypeLink:    TypeLink,
	tar.TypeChar:    TypeChar,
	tar.TypeBlock:   TypeBlock,
	tar.TypeDir:     TypeDir,
	tar.TypeFifo:    TypeFifo,
	tar.TypeSymlink: TypeSymlink,
}

func GetType(t byte) (string, error) {
	r, found := TarTypes[t]
	if !found {
		return "", fmt.Errorf("unknown tarball type: %v", t)
	}
	return r, nil
}

const (
	ManifestChecksumKey = "io.github.containers.zstd-chunked.manifest-checksum"
	ManifestInfoKey     = "io.github.containers.zstd-chunked.manifest-position"
	TarSplitChecksumKey = "io.github.containers.zstd-chunked.tarsplit-checksum"
	TarSplitInfoKey     = "io.github.containers.zstd-chunked.tarsplit-position"

	// ManifestTypeCRFS is a manifest file compatible with the CRFS TOC file.
	ManifestTypeCRFS = 1

	// FooterSizeSupported is the footer size supported by this implementation.
	// Newer versions of the image format might increase this value, so reject
	// any version that is not supported.
	FooterSizeSupported = 56
)

var (
	// when the zstd decoder encounters a skippable frame + 1 byte for the size, it
	// will ignore it.
	// https://tools.ietf.org/html/rfc8478#section-3.1.2
	skippableFrameMagic = []byte{0x50, 0x2a, 0x4d, 0x18}

	ZstdChunkedFrameMagic = []byte{0x47, 0x6e, 0x55, 0x6c, 0x49, 0x6e, 0x55, 0x78}
)

func appendZstdSkippableFrame(dest io.Writer, data []byte) error {
	if _, err := dest.Write(skippableFrameMagic); err != nil {
		return err
	}

	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(data)))
	if _, err := dest.Write(size); err != nil {
		return err
	}
	if _, err := dest.Write(data); err != nil {
		return err
	}
	return nil
}

type TarSplitData struct {
	Data             []byte
	Digest           digest.Digest
	UncompressedSize int64
}

func WriteZstdChunkedManifest(dest io.Writer, outMetadata map[string]string, offset uint64, tarSplitData *TarSplitData, metadata []FileMetadata, level int) error {
	// 8 is the size of the zstd skippable frame header + the frame size
	const zstdSkippableFrameHeader = 8
	manifestOffset := offset + zstdSkippableFrameHeader

	toc := TOC{
		Version: 1,
		Entries: metadata,
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	// Generate the manifest
	manifest, err := json.Marshal(toc)
	if err != nil {
		return err
	}

	var compressedBuffer bytes.Buffer
	zstdWriter, err := ZstdWriterWithLevel(&compressedBuffer, level)
	if err != nil {
		return err
	}
	if _, err := zstdWriter.Write(manifest); err != nil {
		zstdWriter.Close()
		return err
	}
	if err := zstdWriter.Close(); err != nil {
		return err
	}
	compressedManifest := compressedBuffer.Bytes()

	manifestDigester := digest.Canonical.Digester()
	manifestChecksum := manifestDigester.Hash()
	if _, err := manifestChecksum.Write(compressedManifest); err != nil {
		return err
	}

	outMetadata[ManifestChecksumKey] = manifestDigester.Digest().String()
	outMetadata[ManifestInfoKey] = fmt.Sprintf("%d:%d:%d:%d", manifestOffset, len(compressedManifest), len(manifest), ManifestTypeCRFS)
	if err := appendZstdSkippableFrame(dest, compressedManifest); err != nil {
		return err
	}

	outMetadata[TarSplitChecksumKey] = tarSplitData.Digest.String()
	tarSplitOffset := manifestOffset + uint64(len(compressedManifest)) + zstdSkippableFrameHeader
	outMetadata[TarSplitInfoKey] = fmt.Sprintf("%d:%d:%d", tarSplitOffset, len(tarSplitData.Data), tarSplitData.UncompressedSize)
	if err := appendZstdSkippableFrame(dest, tarSplitData.Data); err != nil {
		return err
	}

	// Store the offset to the manifest and its size in LE order
	manifestDataLE := make([]byte, FooterSizeSupported)
	binary.LittleEndian.PutUint64(manifestDataLE, manifestOffset)
	binary.LittleEndian.PutUint64(manifestDataLE[8*1:], uint64(len(compressedManifest)))
	binary.LittleEndian.PutUint64(manifestDataLE[8*2:], uint64(len(manifest)))
	binary.LittleEndian.PutUint64(manifestDataLE[8*3:], uint64(ManifestTypeCRFS))
	copy(manifestDataLE[8*4:], ZstdChunkedFrameMagic)

	return appendZstdSkippableFrame(dest, manifestDataLE)
}

func ZstdWriterWithLevel(dest io.Writer, level int) (*zstd.Encoder, error) {
	el := zstd.EncoderLevelFromZstd(level)
	return zstd.NewWriter(dest, zstd.WithEncoderLevel(el))
}
