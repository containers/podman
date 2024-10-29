package chunked

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vbatts/tar-split/archive/tar"
	"github.com/vbatts/tar-split/tar/storage"
)

// iterateTarSplit calls handler for each tar header in tarSplit
func iterateTarSplit(tarSplit []byte, handler func(hdr *tar.Header) error) error {
	// This, strictly speaking, hard-codes undocumented assumptions about how github.com/vbatts/tar-split/tar/asm.NewInputTarStream
	// forms the tar-split contents. Pragmatically, NewInputTarStream should always produce storage.FileType entries at least
	// for every non-empty file, which constraints it basically to the output we expect.
	//
	// Specifically, we assume:
	// - There is a separate SegmentType entry for every tar header, but only one SegmentType entry for the full header incl. any extensions
	// - (There is a FileType entry for every tar header, we ignore it)
	// - Trailing padding of a file, if any, is included in the next SegmentType entry
	// - At the end, there may be SegmentType entries just for the terminating zero blocks.

	unpacker := storage.NewJSONUnpacker(bytes.NewReader(tarSplit))
	for {
		tsEntry, err := unpacker.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading tar-split entries: %w", err)
		}
		switch tsEntry.Type {
		case storage.SegmentType:
			payload := tsEntry.Payload
			// This is horrible, but we don’t know how much padding to skip. (It can be computed from the previous hdr.Size for non-sparse
			// files, but for sparse files that is set to the logical size.)
			//
			// First, assume that all padding is zero bytes.
			// A tar header starts with a file name, which might in principle be empty, but
			// at least https://github.com/opencontainers/image-spec/blob/main/layer.md#populate-initial-filesystem suggests that
			// the tar name should never be empty (it should be ".", or maybe "./").
			//
			// This will cause us to skip all zero bytes in the trailing blocks, but that’s fine.
			i := 0
			for i < len(payload) && payload[i] == 0 {
				i++
			}
			payload = payload[i:]
			tr := tar.NewReader(bytes.NewReader(payload))
			hdr, err := tr.Next()
			if err != nil {
				if err == io.EOF { // Probably the last entry, but let’s let the unpacker drive that.
					break
				}
				return fmt.Errorf("decoding a tar header from a tar-split entry: %w", err)
			}
			if err := handler(hdr); err != nil {
				return err
			}

		case storage.FileType:
			// Nothing
		default:
			return fmt.Errorf("unexpected tar-split entry type %q", tsEntry.Type)
		}
	}
}
