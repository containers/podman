package chunked

import (
	archivetar "archive/tar"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	"github.com/vbatts/tar-split/archive/tar"
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
		return nil, 0, fmt.Errorf("parse ToC offset: %w", err)
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
// This function uses the io.github.containers.zstd-chunked. annotations when specified.
func readZstdChunkedManifest(blobStream ImageSourceSeekable, blobSize int64, annotations map[string]string) ([]byte, []byte, int64, error) {
	footerSize := int64(internal.FooterSizeSupported)
	if blobSize <= footerSize {
		return nil, nil, 0, errors.New("blob too small")
	}

	var footerData internal.ZstdChunkedFooterData

	if offsetMetadata := annotations[internal.ManifestInfoKey]; offsetMetadata != "" {
		var err error
		footerData, err = internal.ReadFooterDataFromAnnotations(annotations)
		if err != nil {
			return nil, nil, 0, err
		}
	} else {
		chunk := ImageSourceChunk{
			Offset: uint64(blobSize - footerSize),
			Length: uint64(footerSize),
		}
		parts, errs, err := blobStream.GetBlobAt([]ImageSourceChunk{chunk})
		if err != nil {
			return nil, nil, 0, err
		}
		var reader io.ReadCloser
		select {
		case r := <-parts:
			reader = r
		case err := <-errs:
			return nil, nil, 0, err
		}
		footer := make([]byte, footerSize)
		if _, err := io.ReadFull(reader, footer); err != nil {
			return nil, nil, 0, err
		}

		footerData, err = internal.ReadFooterDataFromBlob(footer)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	if footerData.ManifestType != internal.ManifestTypeCRFS {
		return nil, nil, 0, errors.New("invalid manifest type")
	}

	// set a reasonable limit
	if footerData.LengthCompressed > (1<<20)*50 {
		return nil, nil, 0, errors.New("manifest too big")
	}
	if footerData.LengthUncompressed > (1<<20)*50 {
		return nil, nil, 0, errors.New("manifest too big")
	}

	chunk := ImageSourceChunk{
		Offset: footerData.Offset,
		Length: footerData.LengthCompressed,
	}

	chunks := []ImageSourceChunk{chunk}

	if footerData.OffsetTarSplit > 0 {
		chunkTarSplit := ImageSourceChunk{
			Offset: footerData.OffsetTarSplit,
			Length: footerData.LengthCompressedTarSplit,
		}
		chunks = append(chunks, chunkTarSplit)
	}

	parts, errs, err := blobStream.GetBlobAt(chunks)
	if err != nil {
		return nil, nil, 0, err
	}

	readBlob := func(len uint64) ([]byte, error) {
		var reader io.ReadCloser
		select {
		case r := <-parts:
			reader = r
		case err := <-errs:
			return nil, err
		}

		blob := make([]byte, len)
		if _, err := io.ReadFull(reader, blob); err != nil {
			reader.Close()
			return nil, err
		}
		if err := reader.Close(); err != nil {
			return nil, err
		}
		return blob, nil
	}

	manifest, err := readBlob(footerData.LengthCompressed)
	if err != nil {
		return nil, nil, 0, err
	}

	decodedBlob, err := decodeAndValidateBlob(manifest, footerData.LengthUncompressed, footerData.ChecksumAnnotation)
	if err != nil {
		return nil, nil, 0, err
	}
	decodedTarSplit := []byte{}
	if footerData.OffsetTarSplit > 0 {
		tarSplit, err := readBlob(footerData.LengthCompressedTarSplit)
		if err != nil {
			return nil, nil, 0, err
		}

		decodedTarSplit, err = decodeAndValidateBlob(tarSplit, footerData.LengthUncompressedTarSplit, footerData.ChecksumAnnotationTarSplit)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	return decodedBlob, decodedTarSplit, int64(footerData.Offset), err
}

func decodeAndValidateBlob(blob []byte, lengthUncompressed uint64, expectedUncompressedChecksum string) ([]byte, error) {
	d, err := digest.Parse(expectedUncompressedChecksum)
	if err != nil {
		return nil, err
	}

	blobDigester := d.Algorithm().Digester()
	blobChecksum := blobDigester.Hash()
	if _, err := blobChecksum.Write(blob); err != nil {
		return nil, err
	}
	if blobDigester.Digest() != d {
		return nil, fmt.Errorf("invalid blob checksum, expected checksum %s, got %s", d, blobDigester.Digest())
	}

	decoder, err := zstd.NewReader(nil) //nolint:contextcheck
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	b := make([]byte, 0, lengthUncompressed)
	return decoder.DecodeAll(blob, b)
}
