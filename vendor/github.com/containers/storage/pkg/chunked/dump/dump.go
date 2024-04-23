package dump

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/containers/storage/pkg/chunked/internal"
	"golang.org/x/sys/unix"
)

const (
	ESCAPE_STANDARD = 0
	NOESCAPE_SPACE  = 1 << iota
	ESCAPE_EQUAL
	ESCAPE_LONE_DASH
)

func escaped(val string, escape int) string {
	noescapeSpace := escape&NOESCAPE_SPACE != 0
	escapeEqual := escape&ESCAPE_EQUAL != 0
	escapeLoneDash := escape&ESCAPE_LONE_DASH != 0

	length := len(val)

	if escapeLoneDash && val == "-" {
		return fmt.Sprintf("\\x%.2x", val[0])
	}

	var result string
	for i := 0; i < length; i++ {
		c := val[i]
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
				hexEscape = !unicode.IsPrint(rune(c))
			} else {
				hexEscape = !unicode.IsPrint(rune(c)) || unicode.IsSpace(rune(c))
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

func escapedOptional(val string, escape int) string {
	if val == "" {
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

func dumpNode(out io.Writer, links map[string]int, verityDigests map[string]string, entry *internal.FileMetadata) error {
	path := sanitizeName(entry.Name)

	if _, err := fmt.Fprint(out, escaped(path, ESCAPE_STANDARD)); err != nil {
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

	if _, err := fmt.Fprintf(out, escapedOptional(payload, ESCAPE_LONE_DASH)); err != nil {
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
	if _, err := fmt.Fprintf(out, escapedOptional(digest, ESCAPE_LONE_DASH)); err != nil {
		return err
	}

	for k, v := range entry.Xattrs {
		name := escaped(k, ESCAPE_EQUAL)
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
		for _, e := range toc.Entries {
			if e.Linkname == "" {
				continue
			}
			if e.Type == internal.TypeSymlink {
				continue
			}
			links[e.Linkname] = links[e.Linkname] + 1
		}

		if len(toc.Entries) == 0 || (sanitizeName(toc.Entries[0].Name) != "/") {
			root := &internal.FileMetadata{
				Name: "/",
				Type: internal.TypeDir,
				Mode: 0o755,
			}

			if err := dumpNode(w, links, verityDigests, root); err != nil {
				pipeW.CloseWithError(err)
				closed = true
				return
			}
		}

		for _, e := range toc.Entries {
			if e.Type == internal.TypeChunk {
				continue
			}
			if err := dumpNode(w, links, verityDigests, &e); err != nil {
				pipeW.CloseWithError(err)
				closed = true
				return
			}
		}
	}()
	return pipeR, nil
}
