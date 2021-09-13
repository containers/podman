package compressor

// NOTE: This is used from github.com/containers/image by callers that
// don't otherwise use containers/storage, so don't make this depend on any
// larger software like the graph drivers.

import (
	"encoding/base64"
	"io"
	"io/ioutil"

	"github.com/containers/storage/pkg/chunked/internal"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/opencontainers/go-digest"
	"github.com/vbatts/tar-split/archive/tar"
)

func writeZstdChunkedStream(destFile io.Writer, outMetadata map[string]string, reader io.Reader, level int) error {
	// total written so far.  Used to retrieve partial offsets in the file
	dest := ioutils.NewWriteCounter(destFile)

	tr := tar.NewReader(reader)
	tr.RawAccounting = true

	buf := make([]byte, 4096)

	zstdWriter, err := internal.ZstdWriterWithLevel(dest, level)
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

	var metadata []internal.FileMetadata
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

		typ, err := internal.GetType(hdr.Typeflag)
		if err != nil {
			return err
		}
		xattrs := make(map[string]string)
		for k, v := range hdr.Xattrs {
			xattrs[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}
		m := internal.FileMetadata{
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

	return internal.WriteZstdChunkedManifest(dest, outMetadata, uint64(dest.Count), metadata, level)
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

// ZstdCompressor is a CompressorFunc for the zstd compression algorithm.
func ZstdCompressor(r io.Writer, metadata map[string]string, level *int) (io.WriteCloser, error) {
	if level == nil {
		l := 3
		level = &l
	}

	return zstdChunkedWriterWithLevel(r, metadata, *level)
}
