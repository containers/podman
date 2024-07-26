package chunked

import (
	archivetar "archive/tar"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"
	"time"

	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/klauspost/compress/zstd"
	"github.com/klauspost/pgzip"
	digest "github.com/opencontainers/go-digest"
	"github.com/vbatts/tar-split/archive/tar"
	expMaps "golang.org/x/exp/maps"
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

func readEstargzChunkedManifest(blobStream ImageSourceSeekable, blobSize int64, tocDigest digest.Digest) ([]byte, int64, error) {
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

	if manifestDigester.Digest() != tocDigest {
		return nil, 0, errors.New("invalid manifest checksum")
	}

	return manifestUncompressed, tocOffset, nil
}

// readZstdChunkedManifest reads the zstd:chunked manifest from the seekable stream blobStream.
// Returns (manifest blob, parsed manifest, tar-split blob, manifest offset).
func readZstdChunkedManifest(blobStream ImageSourceSeekable, tocDigest digest.Digest, annotations map[string]string) ([]byte, *internal.TOC, []byte, int64, error) {
	offsetMetadata := annotations[internal.ManifestInfoKey]
	if offsetMetadata == "" {
		return nil, nil, nil, 0, fmt.Errorf("%q annotation missing", internal.ManifestInfoKey)
	}
	var manifestChunk ImageSourceChunk
	var manifestLengthUncompressed, manifestType uint64
	if _, err := fmt.Sscanf(offsetMetadata, "%d:%d:%d:%d", &manifestChunk.Offset, &manifestChunk.Length, &manifestLengthUncompressed, &manifestType); err != nil {
		return nil, nil, nil, 0, err
	}
	// The tarSplit… values are valid if tarSplitChunk.Offset > 0
	var tarSplitChunk ImageSourceChunk
	var tarSplitLengthUncompressed uint64
	if tarSplitInfoKeyAnnotation, found := annotations[internal.TarSplitInfoKey]; found {
		if _, err := fmt.Sscanf(tarSplitInfoKeyAnnotation, "%d:%d:%d", &tarSplitChunk.Offset, &tarSplitChunk.Length, &tarSplitLengthUncompressed); err != nil {
			return nil, nil, nil, 0, err
		}
	}

	if manifestType != internal.ManifestTypeCRFS {
		return nil, nil, nil, 0, errors.New("invalid manifest type")
	}

	// set a reasonable limit
	if manifestChunk.Length > (1<<20)*50 {
		return nil, nil, nil, 0, errors.New("manifest too big")
	}
	if manifestLengthUncompressed > (1<<20)*50 {
		return nil, nil, nil, 0, errors.New("manifest too big")
	}

	chunks := []ImageSourceChunk{manifestChunk}
	if tarSplitChunk.Offset > 0 {
		chunks = append(chunks, tarSplitChunk)
	}
	parts, errs, err := blobStream.GetBlobAt(chunks)
	if err != nil {
		return nil, nil, nil, 0, err
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

	manifest, err := readBlob(manifestChunk.Length)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	decodedBlob, err := decodeAndValidateBlob(manifest, manifestLengthUncompressed, tocDigest.String())
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("validating and decompressing TOC: %w", err)
	}
	toc, err := unmarshalToc(decodedBlob)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("unmarshaling TOC: %w", err)
	}

	decodedTarSplit := []byte{}
	if toc.TarSplitDigest != "" {
		if tarSplitChunk.Offset <= 0 {
			return nil, nil, nil, 0, fmt.Errorf("TOC requires a tar-split, but the %s annotation does not describe a position", internal.TarSplitInfoKey)
		}
		tarSplit, err := readBlob(tarSplitChunk.Length)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		decodedTarSplit, err = decodeAndValidateBlob(tarSplit, tarSplitLengthUncompressed, toc.TarSplitDigest.String())
		if err != nil {
			return nil, nil, nil, 0, fmt.Errorf("validating and decompressing tar-split: %w", err)
		}
		// We use the TOC for creating on-disk files, but the tar-split for creating metadata
		// when exporting the layer contents. Ensure the two match, otherwise local inspection of a container
		// might be misleading about the exported contents.
		if err := ensureTOCMatchesTarSplit(toc, decodedTarSplit); err != nil {
			return nil, nil, nil, 0, fmt.Errorf("tar-split and TOC data is inconsistent: %w", err)
		}
	} else if tarSplitChunk.Offset > 0 {
		// We must ignore the tar-split when the digest is not present in the TOC, because we can’t authenticate it.
		//
		// But if we asked for the chunk, now we must consume the data to not block the producer.
		// Ideally the GetBlobAt API should be changed so that this is not necessary.
		_, err := readBlob(tarSplitChunk.Length)
		if err != nil {
			return nil, nil, nil, 0, err
		}
	}
	return decodedBlob, toc, decodedTarSplit, int64(manifestChunk.Offset), err
}

// ensureTOCMatchesTarSplit validates that toc and tarSplit contain _exactly_ the same entries.
func ensureTOCMatchesTarSplit(toc *internal.TOC, tarSplit []byte) error {
	pendingFiles := map[string]*internal.FileMetadata{} // Name -> an entry in toc.Entries
	for i := range toc.Entries {
		e := &toc.Entries[i]
		if e.Type != internal.TypeChunk {
			if _, ok := pendingFiles[e.Name]; ok {
				return fmt.Errorf("TOC contains duplicate entries for path %q", e.Name)
			}
			pendingFiles[e.Name] = e
		}
	}

	if err := iterateTarSplit(tarSplit, func(hdr *tar.Header) error {
		e, ok := pendingFiles[hdr.Name]
		if !ok {
			return fmt.Errorf("tar-split contains an entry for %q missing in TOC", hdr.Name)
		}
		delete(pendingFiles, hdr.Name)
		expected, err := internal.NewFileMetadata(hdr)
		if err != nil {
			return fmt.Errorf("determining expected metadata for %q: %w", hdr.Name, err)
		}
		if err := ensureFileMetadataAttributesMatch(e, &expected); err != nil {
			return fmt.Errorf("TOC and tar-split metadata doesn’t match: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}
	if len(pendingFiles) != 0 {
		remaining := expMaps.Keys(pendingFiles)
		if len(remaining) > 5 {
			remaining = remaining[:5] // Just to limit the size of the output.
		}
		return fmt.Errorf("TOC contains entries not present in tar-split, incl. %q", remaining)
	}
	return nil
}

// ensureTimePointersMatch ensures that a and b are equal
func ensureTimePointersMatch(a, b *time.Time) error {
	// We didn’t always use “timeIfNotZero” when creating the TOC, so treat time.IsZero the same as nil.
	// The archive/tar code turns time.IsZero() timestamps into an Unix timestamp of 0 when writing, but turns an Unix timestamp of 0
	// when writing into a (local-timezone) Jan 1 1970, which is not IsZero(). So, treat that the same as IsZero as well.
	unixZero := time.Unix(0, 0)
	if a != nil && (a.IsZero() || a.Equal(unixZero)) {
		a = nil
	}
	if b != nil && (b.IsZero() || b.Equal(unixZero)) {
		b = nil
	}
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return fmt.Errorf("nil != %v", *b)
	case b == nil:
		return fmt.Errorf("%v != nil", *a)
	default:
		if a.Equal(*b) {
			return nil
		}
		return fmt.Errorf("%v != %v", *a, *b)
	}
}

// ensureFileMetadataAttributesMatch ensures that a and b match in file attributes (it ignores entries relevant to locating data
// in the tar stream or matching contents)
func ensureFileMetadataAttributesMatch(a, b *internal.FileMetadata) error {
	// Keep this in sync with internal.FileMetadata!

	if a.Type != b.Type {
		return fmt.Errorf("mismatch of Type: %q != %q", a.Type, b.Type)
	}
	if a.Name != b.Name {
		return fmt.Errorf("mismatch of Name: %q != %q", a.Name, b.Name)
	}
	if a.Linkname != b.Linkname {
		return fmt.Errorf("mismatch of Linkname: %q != %q", a.Linkname, b.Linkname)
	}
	if a.Mode != b.Mode {
		return fmt.Errorf("mismatch of Mode: %q != %q", a.Mode, b.Mode)
	}
	if a.Size != b.Size {
		return fmt.Errorf("mismatch of Size: %q != %q", a.Size, b.Size)
	}
	if a.UID != b.UID {
		return fmt.Errorf("mismatch of UID: %q != %q", a.UID, b.UID)
	}
	if a.GID != b.GID {
		return fmt.Errorf("mismatch of GID: %q != %q", a.GID, b.GID)
	}

	if err := ensureTimePointersMatch(a.ModTime, b.ModTime); err != nil {
		return fmt.Errorf("mismatch of ModTime: %w", err)
	}
	if err := ensureTimePointersMatch(a.AccessTime, b.AccessTime); err != nil {
		return fmt.Errorf("mismatch of AccessTime: %w", err)
	}
	if err := ensureTimePointersMatch(a.ChangeTime, b.ChangeTime); err != nil {
		return fmt.Errorf("mismatch of ChangeTime: %w", err)
	}
	if a.Devmajor != b.Devmajor {
		return fmt.Errorf("mismatch of Devmajor: %q != %q", a.Devmajor, b.Devmajor)
	}
	if a.Devminor != b.Devminor {
		return fmt.Errorf("mismatch of Devminor: %q != %q", a.Devminor, b.Devminor)
	}
	if !maps.Equal(a.Xattrs, b.Xattrs) {
		return fmt.Errorf("mismatch of Xattrs: %q != %q", a.Xattrs, b.Xattrs)
	}

	// Digest is not compared
	// Offset is not compared
	// EndOffset is not compared

	// ChunkSize is not compared
	// ChunkOffset is not compared
	// ChunkDigest is not compared
	// ChunkType is not compared
	return nil
}

func decodeAndValidateBlob(blob []byte, lengthUncompressed uint64, expectedCompressedChecksum string) ([]byte, error) {
	d, err := digest.Parse(expectedCompressedChecksum)
	if err != nil {
		return nil, fmt.Errorf("invalid digest %q: %w", expectedCompressedChecksum, err)
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
