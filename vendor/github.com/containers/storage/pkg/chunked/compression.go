package chunked

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/containers/storage/pkg/ioutils"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/vbatts/tar-split/archive/tar"
)

type zstdTOC struct {
	Version int                `json:"version"`
	Entries []zstdFileMetadata `json:"entries"`
}

type zstdFileMetadata struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Linkname   string            `json:"linkName,omitempty"`
	Mode       int64             `json:"mode,omitempty"`
	Size       int64             `json:"size"`
	UID        int               `json:"uid"`
	GID        int               `json:"gid"`
	ModTime    time.Time         `json:"modtime"`
	AccessTime time.Time         `json:"accesstime"`
	ChangeTime time.Time         `json:"changetime"`
	Devmajor   int64             `json:"devMajor"`
	Devminor   int64             `json:"devMinor"`
	Xattrs     map[string]string `json:"xattrs,omitempty"`
	Digest     string            `json:"digest,omitempty"`
	Offset     int64             `json:"offset,omitempty"`
	EndOffset  int64             `json:"endOffset,omitempty"`

	// Currently chunking is not supported.
	ChunkSize   int64  `json:"chunkSize,omitempty"`
	ChunkOffset int64  `json:"chunkOffset,omitempty"`
	ChunkDigest string `json:"chunkDigest,omitempty"`
}

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

var tarTypes = map[byte]string{
	tar.TypeReg:     TypeReg,
	tar.TypeRegA:    TypeReg,
	tar.TypeLink:    TypeLink,
	tar.TypeChar:    TypeChar,
	tar.TypeBlock:   TypeBlock,
	tar.TypeDir:     TypeDir,
	tar.TypeFifo:    TypeFifo,
	tar.TypeSymlink: TypeSymlink,
}

var typesToTar = map[string]byte{
	TypeReg:     tar.TypeReg,
	TypeLink:    tar.TypeLink,
	TypeChar:    tar.TypeChar,
	TypeBlock:   tar.TypeBlock,
	TypeDir:     tar.TypeDir,
	TypeFifo:    tar.TypeFifo,
	TypeSymlink: tar.TypeSymlink,
}

func getType(t byte) (string, error) {
	r, found := tarTypes[t]
	if !found {
		return "", fmt.Errorf("unknown tarball type: %v", t)
	}
	return r, nil
}

func typeToTarType(t string) (byte, error) {
	r, found := typesToTar[t]
	if !found {
		return 0, fmt.Errorf("unknown type: %v", t)
	}
	return r, nil
}

const (
	manifestChecksumKey = "io.containers.zstd-chunked.manifest-checksum"
	manifestInfoKey     = "io.containers.zstd-chunked.manifest-position"

	// manifestTypeCRFS is a manifest file compatible with the CRFS TOC file.
	manifestTypeCRFS = 1

	// footerSizeSupported is the footer size supported by this implementation.
	// Newer versions of the image format might increase this value, so reject
	// any version that is not supported.
	footerSizeSupported = 40
)

var (
	// when the zstd decoder encounters a skippable frame + 1 byte for the size, it
	// will ignore it.
	// https://tools.ietf.org/html/rfc8478#section-3.1.2
	skippableFrameMagic = []byte{0x50, 0x2a, 0x4d, 0x18}

	zstdChunkedFrameMagic = []byte{0x47, 0x6e, 0x55, 0x6c, 0x49, 0x6e, 0x55, 0x78}
)

func isZstdChunkedFrameMagic(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.Equal(zstdChunkedFrameMagic, data[:8])
}

// readZstdChunkedManifest reads the zstd:chunked manifest from the seekable stream blobStream.  The blob total size must
// be specified.
// This function uses the io.containers.zstd-chunked. annotations when specified.
func readZstdChunkedManifest(blobStream ImageSourceSeekable, blobSize int64, annotations map[string]string) ([]byte, error) {
	footerSize := int64(footerSizeSupported)
	if blobSize <= footerSize {
		return nil, errors.New("blob too small")
	}

	manifestChecksumAnnotation := annotations[manifestChecksumKey]
	if manifestChecksumAnnotation == "" {
		return nil, fmt.Errorf("manifest checksum annotation %q not found", manifestChecksumKey)
	}

	var offset, length, lengthUncompressed, manifestType uint64

	if offsetMetadata := annotations[manifestInfoKey]; offsetMetadata != "" {
		if _, err := fmt.Sscanf(offsetMetadata, "%d:%d:%d:%d", &offset, &length, &lengthUncompressed, &manifestType); err != nil {
			return nil, err
		}
	} else {
		chunk := ImageSourceChunk{
			Offset: uint64(blobSize - footerSize),
			Length: uint64(footerSize),
		}
		parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
		if err != nil {
			return nil, err
		}
		var reader io.ReadCloser
		select {
		case r := <-parts:
			reader = r
		case err := <-errs:
			return nil, err
		}
		footer := make([]byte, footerSize)
		if _, err := io.ReadFull(reader, footer); err != nil {
			return nil, err
		}

		offset = binary.LittleEndian.Uint64(footer[0:8])
		length = binary.LittleEndian.Uint64(footer[8:16])
		lengthUncompressed = binary.LittleEndian.Uint64(footer[16:24])
		manifestType = binary.LittleEndian.Uint64(footer[24:32])
		if !isZstdChunkedFrameMagic(footer[32:40]) {
			return nil, errors.New("invalid magic number")
		}
	}

	if manifestType != manifestTypeCRFS {
		return nil, errors.New("invalid manifest type")
	}

	// set a reasonable limit
	if length > (1<<20)*50 {
		return nil, errors.New("manifest too big")
	}
	if lengthUncompressed > (1<<20)*50 {
		return nil, errors.New("manifest too big")
	}

	chunk := ImageSourceChunk{
		Offset: offset,
		Length: length,
	}

	parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
	if err != nil {
		return nil, err
	}
	var reader io.ReadCloser
	select {
	case r := <-parts:
		reader = r
	case err := <-errs:
		return nil, err
	}

	manifest := make([]byte, length)
	if _, err := io.ReadFull(reader, manifest); err != nil {
		return nil, err
	}

	manifestDigester := digest.Canonical.Digester()
	manifestChecksum := manifestDigester.Hash()
	if _, err := manifestChecksum.Write(manifest); err != nil {
		return nil, err
	}

	d, err := digest.Parse(manifestChecksumAnnotation)
	if err != nil {
		return nil, err
	}
	if manifestDigester.Digest() != d {
		return nil, errors.New("invalid manifest checksum")
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	b := make([]byte, 0, lengthUncompressed)
	if decoded, err := decoder.DecodeAll(manifest, b); err == nil {
		return decoded, nil
	}

	return manifest, nil
}

func appendZstdSkippableFrame(dest io.Writer, data []byte) error {
	if _, err := dest.Write(skippableFrameMagic); err != nil {
		return err
	}

	var size []byte = make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(data)))
	if _, err := dest.Write(size); err != nil {
		return err
	}
	if _, err := dest.Write(data); err != nil {
		return err
	}
	return nil
}

func writeZstdChunkedManifest(dest io.Writer, outMetadata map[string]string, offset uint64, metadata []zstdFileMetadata, level int) error {
	// 8 is the size of the zstd skippable frame header + the frame size
	manifestOffset := offset + 8

	toc := zstdTOC{
		Version: 1,
		Entries: metadata,
	}

	// Generate the manifest
	manifest, err := json.Marshal(toc)
	if err != nil {
		return err
	}

	var compressedBuffer bytes.Buffer
	zstdWriter, err := zstdWriterWithLevel(&compressedBuffer, level)
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

	outMetadata[manifestChecksumKey] = manifestDigester.Digest().String()
	outMetadata[manifestInfoKey] = fmt.Sprintf("%d:%d:%d:%d", manifestOffset, len(compressedManifest), len(manifest), manifestTypeCRFS)
	if err := appendZstdSkippableFrame(dest, compressedManifest); err != nil {
		return err
	}

	// Store the offset to the manifest and its size in LE order
	var manifestDataLE []byte = make([]byte, footerSizeSupported)
	binary.LittleEndian.PutUint64(manifestDataLE, manifestOffset)
	binary.LittleEndian.PutUint64(manifestDataLE[8:], uint64(len(compressedManifest)))
	binary.LittleEndian.PutUint64(manifestDataLE[16:], uint64(len(manifest)))
	binary.LittleEndian.PutUint64(manifestDataLE[24:], uint64(manifestTypeCRFS))
	copy(manifestDataLE[32:], zstdChunkedFrameMagic)

	return appendZstdSkippableFrame(dest, manifestDataLE)
}

func writeZstdChunkedStream(destFile io.Writer, outMetadata map[string]string, reader io.Reader, level int) error {
	// total written so far.  Used to retrieve partial offsets in the file
	dest := ioutils.NewWriteCounter(destFile)

	tr := tar.NewReader(reader)
	tr.RawAccounting = true

	buf := make([]byte, 4096)

	zstdWriter, err := zstdWriterWithLevel(dest, level)
	if err != nil {
		return err
	}
	defer func() {
		if zstdWriter != nil {
			zstdWriter.Close()
			zstdWriter.Flush()
		}
	}()

	restartCompression := func() (int64, error) {
		var offset int64
		if zstdWriter != nil {
			if err := zstdWriter.Close(); err != nil {
				return 0, err
			}
			if err := zstdWriter.Flush(); err != nil {
				return 0, err
			}
			offset = dest.Count
			zstdWriter.Reset(dest)
		}
		return offset, nil
	}

	var metadata []zstdFileMetadata
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		rawBytes := tr.RawBytes()
		if _, err := zstdWriter.Write(rawBytes); err != nil {
			return err
		}
		payloadDigester := digest.Canonical.Digester()
		payloadChecksum := payloadDigester.Hash()

		payloadDest := io.MultiWriter(payloadChecksum, zstdWriter)

		// Now handle the payload, if any
		var startOffset, endOffset int64
		checksum := ""
		for {
			read, errRead := tr.Read(buf)
			if errRead != nil && errRead != io.EOF {
				return err
			}

			// restart the compression only if there is
			// a payload.
			if read > 0 {
				if startOffset == 0 {
					startOffset, err = restartCompression()
					if err != nil {
						return err
					}
				}
				_, err := payloadDest.Write(buf[:read])
				if err != nil {
					return err
				}
			}
			if errRead == io.EOF {
				if startOffset > 0 {
					endOffset, err = restartCompression()
					if err != nil {
						return err
					}
					checksum = payloadDigester.Digest().String()
				}
				break
			}
		}

		typ, err := getType(hdr.Typeflag)
		if err != nil {
			return err
		}
		xattrs := make(map[string]string)
		for k, v := range hdr.Xattrs {
			xattrs[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}
		m := zstdFileMetadata{
			Type:       typ,
			Name:       hdr.Name,
			Linkname:   hdr.Linkname,
			Mode:       hdr.Mode,
			Size:       hdr.Size,
			UID:        hdr.Uid,
			GID:        hdr.Gid,
			ModTime:    hdr.ModTime,
			AccessTime: hdr.AccessTime,
			ChangeTime: hdr.ChangeTime,
			Devmajor:   hdr.Devmajor,
			Devminor:   hdr.Devminor,
			Xattrs:     xattrs,
			Digest:     checksum,
			Offset:     startOffset,
			EndOffset:  endOffset,

			// ChunkSize is 0 for the last chunk
			ChunkSize:   0,
			ChunkOffset: 0,
			ChunkDigest: checksum,
		}
		metadata = append(metadata, m)
	}

	rawBytes := tr.RawBytes()
	if _, err := zstdWriter.Write(rawBytes); err != nil {
		return err
	}
	if err := zstdWriter.Flush(); err != nil {
		return err
	}
	if err := zstdWriter.Close(); err != nil {
		return err
	}
	zstdWriter = nil

	return writeZstdChunkedManifest(dest, outMetadata, uint64(dest.Count), metadata, level)
}

type zstdChunkedWriter struct {
	tarSplitOut *io.PipeWriter
	tarSplitErr chan error
}

func (w zstdChunkedWriter) Close() error {
	err := <-w.tarSplitErr
	if err != nil {
		w.tarSplitOut.Close()
		return err
	}
	return w.tarSplitOut.Close()
}

func (w zstdChunkedWriter) Write(p []byte) (int, error) {
	select {
	case err := <-w.tarSplitErr:
		w.tarSplitOut.Close()
		return 0, err
	default:
		return w.tarSplitOut.Write(p)
	}
}

// zstdChunkedWriterWithLevel writes a zstd compressed tarball where each file is
// compressed separately so it can be addressed separately.  Idea based on CRFS:
// https://github.com/google/crfs
// The difference with CRFS is that the zstd compression is used instead of gzip.
// The reason for it is that zstd supports embedding metadata ignored by the decoder
// as part of the compressed stream.
// A manifest json file with all the metadata is appended at the end of the tarball
// stream, using zstd skippable frames.
// The final file will look like:
// [FILE_1][FILE_2]..[FILE_N][SKIPPABLE FRAME 1][SKIPPABLE FRAME 2]
// Where:
// [FILE_N]: [ZSTD HEADER][TAR HEADER][PAYLOAD FILE_N][ZSTD FOOTER]
// [SKIPPABLE FRAME 1]: [ZSTD SKIPPABLE FRAME, SIZE=MANIFEST LENGTH][MANIFEST]
// [SKIPPABLE FRAME 2]: [ZSTD SKIPPABLE FRAME, SIZE=16][MANIFEST_OFFSET][MANIFEST_LENGTH][MANIFEST_LENGTH_UNCOMPRESSED][MANIFEST_TYPE][CHUNKED_ZSTD_MAGIC_NUMBER]
// MANIFEST_OFFSET, MANIFEST_LENGTH, MANIFEST_LENGTH_UNCOMPRESSED and CHUNKED_ZSTD_MAGIC_NUMBER are 64 bits unsigned in little endian format.
func zstdChunkedWriterWithLevel(out io.Writer, metadata map[string]string, level int) (io.WriteCloser, error) {
	ch := make(chan error, 1)
	r, w := io.Pipe()

	go func() {
		ch <- writeZstdChunkedStream(out, metadata, r, level)
		io.Copy(ioutil.Discard, r)
		r.Close()
		close(ch)
	}()

	return zstdChunkedWriter{
		tarSplitOut: w,
		tarSplitErr: ch,
	}, nil
}

func zstdWriterWithLevel(dest io.Writer, level int) (*zstd.Encoder, error) {
	el := zstd.EncoderLevelFromZstd(level)
	return zstd.NewWriter(dest, zstd.WithEncoderLevel(el))
}

// ZstdCompressor is a CompressorFunc for the zstd compression algorithm.
func ZstdCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	if level == nil {
		l := 3
		level = &l
	}

	return zstdChunkedWriterWithLevel(r, metadata, *level)
}
