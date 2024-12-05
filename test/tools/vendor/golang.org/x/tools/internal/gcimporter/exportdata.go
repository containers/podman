// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is a copy of $GOROOT/src/go/internal/gcimporter/exportdata.go.

// This file implements FindExportData.

package gcimporter

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readGopackHeader(r *bufio.Reader) (name string, size int64, err error) {
	// See $GOROOT/include/ar.h.
	hdr := make([]byte, 16+12+6+6+8+10+2)
	_, err = io.ReadFull(r, hdr)
	if err != nil {
		return
	}
	// leave for debugging
	if false {
		fmt.Printf("header: %s", hdr)
	}
	s := strings.TrimSpace(string(hdr[16+12+6+6+8:][:10]))
	length, err := strconv.Atoi(s)
	size = int64(length)
	if err != nil || hdr[len(hdr)-2] != '`' || hdr[len(hdr)-1] != '\n' {
		err = fmt.Errorf("invalid archive header")
		return
	}
	name = strings.TrimSpace(string(hdr[:16]))
	return
}

// FindExportData positions the reader r at the beginning of the
// export data section of an underlying cmd/compile created archive
// file by reading from it. The reader must be positioned at the
// start of the file before calling this function.
// The size result is the length of the export data in bytes.
//
// This function is needed by [gcexportdata.Read], which must
// accept inputs produced by the last two releases of cmd/compile,
// plus tip.
func FindExportData(r *bufio.Reader) (size int64, err error) {
	// Read first line to make sure this is an object file.
	line, err := r.ReadSlice('\n')
	if err != nil {
		err = fmt.Errorf("can't find export data (%v)", err)
		return
	}

	// Is the first line an archive file signature?
	if string(line) != "!<arch>\n" {
		err = fmt.Errorf("not the start of an archive file (%q)", line)
		return
	}

	// Archive file. Scan to __.PKGDEF.
	var name string
	if name, size, err = readGopackHeader(r); err != nil {
		return
	}
	arsize := size

	// First entry should be __.PKGDEF.
	if name != "__.PKGDEF" {
		err = fmt.Errorf("go archive is missing __.PKGDEF")
		return
	}

	// Read first line of __.PKGDEF data, so that line
	// is once again the first line of the input.
	if line, err = r.ReadSlice('\n'); err != nil {
		err = fmt.Errorf("can't find export data (%v)", err)
		return
	}
	size -= int64(len(line))

	// Now at __.PKGDEF in archive or still at beginning of file.
	// Either way, line should begin with "go object ".
	if !strings.HasPrefix(string(line), "go object ") {
		err = fmt.Errorf("not a Go object file")
		return
	}

	// Skip over object headers to get to the export data section header "$$B\n".
	// Object headers are lines that do not start with '$'.
	for line[0] != '$' {
		if line, err = r.ReadSlice('\n'); err != nil {
			err = fmt.Errorf("can't find export data (%v)", err)
			return
		}
		size -= int64(len(line))
	}

	// Check for the binary export data section header "$$B\n".
	hdr := string(line)
	if hdr != "$$B\n" {
		err = fmt.Errorf("unknown export data header: %q", hdr)
		return
	}
	// TODO(taking): Remove end-of-section marker "\n$$\n" from size.

	if size < 0 {
		err = fmt.Errorf("invalid size (%d) in the archive file: %d bytes remain without section headers (recompile package)", arsize, size)
		return
	}

	return
}
