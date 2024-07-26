//go:build unix

package dump

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/containers/storage/pkg/chunked/internal"
	"golang.org/x/sys/unix"
)

const (
	ESCAPE_STANDARD = 0
	NOESCAPE_SPACE  = 1 << iota
	ESCAPE_EQUAL
	ESCAPE_LONE_DASH
)

func escaped(val []byte, escape int) string {
	noescapeSpace := escape&NOESCAPE_SPACE != 0
	escapeEqual := escape&ESCAPE_EQUAL != 0
	escapeLoneDash := escape&ESCAPE_LONE_DASH != 0

	if escapeLoneDash && len(val) == 1 && val[0] == '-' {
		return fmt.Sprintf("\\x%.2x", val[0])
	}

	// This is intended to match the C isprint API with LC_CTYPE=C
	isprint := func(c byte) bool {
		return c >= 32 && c < 127
	}
	// This is intended to match the C isgraph API with LC_CTYPE=C
	isgraph := func(c byte) bool {
		return c > 32 && c < 127
	}

	var result string
	for _, c := range []byte(val) {
		hexEscape := false
		var special string

		switch c {
		case '\\':
			special = "\\\\"
		case '\n':
			special = "\\n"
		case '\r':
			special = "\\r"
		case '\t':
			special = "\\t"
		case '=':
			hexEscape = escapeEqual
		default:
			if noescapeSpace {
				hexEscape = !isprint(c)
			} else {
				hexEscape = !isgraph(c)
			}
		}

		if special != "" {
			result += special
		} else if hexEscape {
			result += fmt.Sprintf("\\x%.2x", c)
		} else {
			result += string(c)
		}
	}
	return result
}

func escapedOptional(val []byte, escape int) string {
	if len(val) == 0 {
		return "-"
	}
	return escaped(val, escape)
}

func getStMode(mode uint32, typ string) (uint32, error) {
	switch typ {
	case internal.TypeReg, internal.TypeLink:
		mode |= unix.S_IFREG
	case internal.TypeChar:
		mode |= unix.S_IFCHR
	case internal.TypeBlock:
		mode |= unix.S_IFBLK
	case internal.TypeDir:
		mode |= unix.S_IFDIR
	case internal.TypeFifo:
		mode |= unix.S_IFIFO
	case internal.TypeSymlink:
		mode |= unix.S_IFLNK
	default:
		return 0, fmt.Errorf("unknown type %s", typ)
	}
	return mode, nil
}

func sanitizeName(name string) string {
	path := filepath.Clean(name)
	if path == "." {
		path = "/"
	} else if path[0] != '/' {
		path = "/" + path
	}
	return path
}

func dumpNode(out io.Writer, added map[string]*internal.FileMetadata, links map[string]int, verityDigests map[string]string, entry *internal.FileMetadata) error {
	path := sanitizeName(entry.Name)

	parent := filepath.Dir(path)
	if _, found := added[parent]; !found && path != "/" {
		parentEntry := &internal.FileMetadata{
			Name: parent,
			Type: internal.TypeDir,
			Mode: 0o755,
		}
		if err := dumpNode(out, added, links, verityDigests, parentEntry); err != nil {
			return err
		}

	}
	if e, found := added[path]; found {
		// if the entry was already added, make sure it has the same data
		if !reflect.DeepEqual(*e, *entry) {
			return fmt.Errorf("entry %q already added with different data", path)
		}
		return nil
	}
	added[path] = entry

	if _, err := fmt.Fprint(out, escaped([]byte(path), ESCAPE_STANDARD)); err != nil {
		return err
	}

	nlinks := links[entry.Name] + links[entry.Linkname] + 1
	link := ""
	if entry.Type == internal.TypeLink {
		link = "@"
	}

	rdev := unix.Mkdev(uint32(entry.Devmajor), uint32(entry.Devminor))

	entryTime := entry.ModTime
	if entryTime == nil {
		t := time.Unix(0, 0)
		entryTime = &t
	}

	mode, err := getStMode(uint32(entry.Mode), entry.Type)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, " %d %s%o %d %d %d %d %d.%d ", entry.Size,
		link, mode,
		nlinks, entry.UID, entry.GID, rdev,
		entryTime.Unix(), entryTime.Nanosecond()); err != nil {
		return err
	}

	var payload string
	if entry.Linkname != "" {
		if entry.Type == internal.TypeSymlink {
			payload = entry.Linkname
		} else {
			payload = sanitizeName(entry.Linkname)
		}
	} else {
		if len(entry.Digest) > 10 {
			d := strings.Replace(entry.Digest, "sha256:", "", 1)
			payload = d[:2] + "/" + d[2:]
		}
	}

	if _, err := fmt.Fprint(out, escapedOptional([]byte(payload), ESCAPE_LONE_DASH)); err != nil {
		return err
	}

	/* inline content.  */
	if _, err := fmt.Fprint(out, " -"); err != nil {
		return err
	}

	/* store the digest.  */
	if _, err := fmt.Fprint(out, " "); err != nil {
		return err
	}
	digest := verityDigests[payload]
	if _, err := fmt.Fprint(out, escapedOptional([]byte(digest), ESCAPE_LONE_DASH)); err != nil {
		return err
	}

	for k, vEncoded := range entry.Xattrs {
		v, err := base64.StdEncoding.DecodeString(vEncoded)
		if err != nil {
			return fmt.Errorf("decode xattr %q: %w", k, err)
		}
		name := escaped([]byte(k), ESCAPE_EQUAL)

		value := escaped(v, ESCAPE_EQUAL)
		if _, err := fmt.Fprintf(out, " %s=%s", name, value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(out, "\n"); err != nil {
		return err
	}
	return nil
}

// GenerateDump generates a dump of the TOC in the same format as `composefs-info dump`
func GenerateDump(tocI interface{}, verityDigests map[string]string) (io.Reader, error) {
	toc, ok := tocI.(*internal.TOC)
	if !ok {
		return nil, fmt.Errorf("invalid TOC type")
	}
	pipeR, pipeW := io.Pipe()
	go func() {
		closed := false
		w := bufio.NewWriter(pipeW)
		defer func() {
			if !closed {
				w.Flush()
				pipeW.Close()
			}
		}()

		links := make(map[string]int)
		added := make(map[string]*internal.FileMetadata)
		for _, e := range toc.Entries {
			if e.Linkname == "" {
				continue
			}
			if e.Type == internal.TypeSymlink {
				continue
			}
			links[e.Linkname] = links[e.Linkname] + 1
		}

		if len(toc.Entries) == 0 {
			root := &internal.FileMetadata{
				Name: "/",
				Type: internal.TypeDir,
				Mode: 0o755,
			}

			if err := dumpNode(w, added, links, verityDigests, root); err != nil {
				pipeW.CloseWithError(err)
				closed = true
				return
			}
		}

		for _, e := range toc.Entries {
			if e.Type == internal.TypeChunk {
				continue
			}
			if err := dumpNode(w, added, links, verityDigests, &e); err != nil {
				pipeW.CloseWithError(err)
				closed = true
				return
			}
		}
	}()
	return pipeR, nil
}
