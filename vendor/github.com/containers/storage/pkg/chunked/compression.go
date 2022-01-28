package chunked

import (
	archivetar "archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containers/storage/pkg/chunked/compressor"
	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/vbatts/tar-split/archive/tar"
)

const (
	TypeReg     = internal.TypeReg
	TypeChunk   = internal.TypeChunk
	TypeLink    = internal.TypeLink
	TypeChar    = internal.TypeChar
	TypeBlock   = internal.TypeBlock
	TypeDir     = internal.TypeDir
	TypeFifo    = internal.TypeFifo
	TypeSymlink = internal.TypeSymlink
)

var typesToTar = map[string]byte{
	TypeReg:     tar.TypeReg,
	TypeLink:    tar.TypeLink,
	TypeChar:    tar.TypeChar,
	TypeBlock:   tar.TypeBlock,
	TypeDir:     tar.TypeDir,
	TypeFifo:    tar.TypeFifo,
	TypeSymlink: tar.TypeSymlink,
}

func typeToTarType(t string) (byte, error) {
	r, found := typesToTar[t]
	if !found {
		return 0, fmt.Errorf("unknown type: %v", t)
	}
	return r, nil
}

func isZstdChunkedFrameMagic(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	return bytes.Equal(internal.ZstdChunkedFrameMagic, data[:8])
}

func readEstargzChunkedManifest(blobStream ImageSourceSeekable, blobSize int64, annotations map[string]string) ([]byte, int64, error) {
	// information on the format here https://github.com/containerd/stargz-snapshotter/blob/main/docs/stargz-estargz.md
	footerSize := int64(51)
	if blobSize <= footerSize {
		return nil, 0, errors.New("blob too small")
	}
	chunk := ImageSourceChunk{
		Offset: uint64(blobSize - footerSize),
		Length: uint64(footerSize),
	}
	parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
	if err != nil {
		return nil, 0, err
	}
	var reader io.ReadCloser
	select {
	case r := <-parts:
		reader = r
	case err := <-errs:
		return nil, 0, err
	}
	defer reader.Close()
	footer := make([]byte, footerSize)
	if _, err := io.ReadFull(reader, footer); err != nil {
		return nil, 0, err
	}

	/* Read the ToC offset:
	   - 10 bytes  gzip header
	   - 2  bytes  XLEN (length of Extra field) = 26 (4 bytes header + 16 hex digits + len("STARGZ"))
	   - 2  bytes  Extra: SI1 = 'S', SI2 = 'G'
	   - 2  bytes  Extra: LEN = 22 (16 hex digits + len("STARGZ"))
	   - 22 bytes  Extra: subfield = fmt.Sprintf("%016xSTARGZ", offsetOfTOC)
	   - 5  bytes  flate header: BFINAL = 1(last block), BTYPE = 0(non-compressed block), LEN = 0
	   - 8  bytes  gzip footer
	*/
	tocOffset, err := strconv.ParseInt(string(footer[16:16+22-6]), 16, 64)
	if err != nil {
		return nil, 0, errors.Wrap(err, "parse ToC offset")
	}

	size := int64(blobSize - footerSize - tocOffset)
	// set a reasonable limit
	if size > (1<<20)*50 {
		return nil, 0, errors.New("manifest too big")
	}

	chunk = ImageSourceChunk{
		Offset: uint64(tocOffset),
		Length: uint64(size),
	}
	parts, errs, err = blobStream.GetBlobAt([]ImageSourceChunk{chunk})
	if err != nil {
		return nil, 0, err
	}

	var tocReader io.ReadCloser
	select {
	case r := <-parts:
		tocReader = r
	case err := <-errs:
		return nil, 0, err
	}
	defer tocReader.Close()

	r, err := pgzip.NewReader(tocReader)
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()

	aTar := archivetar.NewReader(r)

	header, err := aTar.Next()
	if err != nil {
		return nil, 0, err
	}
	// set a reasonable limit
	if header.Size > (1<<20)*50 {
		return nil, 0, errors.New("manifest too big")
	}

	manifestUncompressed := make([]byte, header.Size)
	if _, err := io.ReadFull(aTar, manifestUncompressed); err != nil {
		return nil, 0, err
	}

	manifestDigester := digest.Canonical.Digester()
	manifestChecksum := manifestDigester.Hash()
	if _, err := manifestChecksum.Write(manifestUncompressed); err != nil {
		return nil, 0, err
	}

	d, err := digest.Parse(annotations[estargz.TOCJSONDigestAnnotation])
	if err != nil {
		return nil, 0, err
	}
	if manifestDigester.Digest() != d {
		return nil, 0, errors.New("invalid manifest checksum")
	}

	return manifestUncompressed, tocOffset, nil
}

// readZstdChunkedManifest reads the zstd:chunked manifest from the seekable stream blobStream.  The blob total size must
// be specified.
// This function uses the io.containers.zstd-chunked. annotations when specified.
func readZstdChunkedManifest(blobStream ImageSourceSeekable, blobSize int64, annotations map[string]string) ([]byte, int64, error) {
	footerSize := int64(internal.FooterSizeSupported)
	if blobSize <= footerSize {
		return nil, 0, errors.New("blob too small")
	}

	manifestChecksumAnnotation := annotations[internal.ManifestChecksumKey]
	if manifestChecksumAnnotation == "" {
		return nil, 0, fmt.Errorf("manifest checksum annotation %q not found", internal.ManifestChecksumKey)
	}

	var offset, length, lengthUncompressed, manifestType uint64

	if offsetMetadata := annotations[internal.ManifestInfoKey]; offsetMetadata != "" {
		if _, err := fmt.Sscanf(offsetMetadata, "%d:%d:%d:%d", &offset, &length, &lengthUncompressed, &manifestType); err != nil {
			return nil, 0, err
		}
	} else {
		chunk := ImageSourceChunk{
			Offset: uint64(blobSize - footerSize),
			Length: uint64(footerSize),
		}
		parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
		if err != nil {
			return nil, 0, err
		}
		var reader io.ReadCloser
		select {
		case r := <-parts:
			reader = r
		case err := <-errs:
			return nil, 0, err
		}
		footer := make([]byte, footerSize)
		if _, err := io.ReadFull(reader, footer); err != nil {
			return nil, 0, err
		}

		offset = binary.LittleEndian.Uint64(footer[0:8])
		length = binary.LittleEndian.Uint64(footer[8:16])
		lengthUncompressed = binary.LittleEndian.Uint64(footer[16:24])
		manifestType = binary.LittleEndian.Uint64(footer[24:32])
		if !isZstdChunkedFrameMagic(footer[32:40]) {
			return nil, 0, errors.New("invalid magic number")
		}
	}

	if manifestType != internal.ManifestTypeCRFS {
		return nil, 0, errors.New("invalid manifest type")
	}

	// set a reasonable limit
	if length > (1<<20)*50 {
		return nil, 0, errors.New("manifest too big")
	}
	if lengthUncompressed > (1<<20)*50 {
		return nil, 0, errors.New("manifest too big")
	}

	chunk := ImageSourceChunk{
		Offset: offset,
		Length: length,
	}

	parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
	if err != nil {
		return nil, 0, err
	}
	var reader io.ReadCloser
	select {
	case r := <-parts:
		reader = r
	case err := <-errs:
		return nil, 0, err
	}

	manifest := make([]byte, length)
	if _, err := io.ReadFull(reader, manifest); err != nil {
		return nil, 0, err
	}

	manifestDigester := digest.Canonical.Digester()
	manifestChecksum := manifestDigester.Hash()
	if _, err := manifestChecksum.Write(manifest); err != nil {
		return nil, 0, err
	}

	d, err := digest.Parse(manifestChecksumAnnotation)
	if err != nil {
		return nil, 0, err
	}
	if manifestDigester.Digest() != d {
		return nil, 0, errors.New("invalid manifest checksum")
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, 0, err
	}
	defer decoder.Close()

	b := make([]byte, 0, lengthUncompressed)
	if decoded, err := decoder.DecodeAll(manifest, b); err == nil {
		return decoded, int64(offset), nil
	}

	return manifest, int64(offset), nil
}

// ZstdCompressor is a CompressorFunc for the zstd compression algorithm.
// Deprecated: Use pkg/chunked/compressor.ZstdCompressor.
func ZstdCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	return compressor.ZstdCompressor(r, metadata, level)
}
