package otbuiltin

import (
	"fmt"
	"time"
	"unsafe"

	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
)

// #cgo pkg-config: ostree-1
// #include <stdlib.h>
// #include <glib.h>
// #include <ostree.h>
// #include "builtin.go.h"
import "C"

// LogEntry is a struct for the various pieces of data in a log entry
type LogEntry struct {
	Checksum  []byte
	Variant   []byte
	Timestamp time.Time
	Subject   string
	Body      string
}

// Convert the log entry to a string
func (l LogEntry) String() string {
	if len(l.Variant) == 0 {
		return fmt.Sprintf("%s\n%s\n\n\t%s\n\n\t%s\n\n", l.Checksum, l.Timestamp, l.Subject, l.Body)
	}
	return fmt.Sprintf("%s\n%s\n\n", l.Checksum, l.Variant)
}

type ostreeDumpFlags uint

const (
	ostreeDumpNone ostreeDumpFlags = 0
	ostreeDumpRaw  ostreeDumpFlags = 1 << iota
)

// logOptions contains all of the options for initializing an ostree repo
type logOptions struct {
	// Raw determines whether to show raw variant data
	Raw bool
}

// NewLogOptions instantiates and returns a logOptions struct with default values set
func NewLogOptions() logOptions {
	return logOptions{}
}

// Log shows the logs of a branch starting with a given commit or ref.  Returns a
// slice of log entries on success and an error otherwise
func Log(repoPath, branch string, options logOptions) ([]LogEntry, error) {
	// attempt to open the repository
	repo, err := OpenRepo(repoPath)
	if err != nil {
		return nil, err
	}

	cbranch := C.CString(branch)
	defer C.free(unsafe.Pointer(cbranch))
	var checksum *C.char
	defer C.free(unsafe.Pointer(checksum))
	var cerr *C.GError
	defer C.free(unsafe.Pointer(cerr))

	flags := ostreeDumpNone
	if options.Raw {
		flags |= ostreeDumpRaw
	}

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.native(), cbranch, C.FALSE, &checksum, &cerr))) {
		return nil, generateError(cerr)
	}

	return logCommit(repo, checksum, false, flags)
}

func logCommit(repo *Repo, checksum *C.char, isRecursive bool, flags ostreeDumpFlags) ([]LogEntry, error) {
	var variant *C.GVariant
	var gerr = glib.NewGError()
	var cerr = (*C.GError)(gerr.Ptr())
	defer C.free(unsafe.Pointer(cerr))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_load_variant(repo.native(), C.OSTREE_OBJECT_TYPE_COMMIT, checksum, &variant, &cerr))) {
		if isRecursive && glib.GoBool(glib.GBoolean(C.g_error_matches(cerr, C.g_io_error_quark(), C.G_IO_ERROR_NOT_FOUND))) {
			return nil, nil
		}
		return nil, generateError(cerr)
	}

	// Get the parent of this commit
	parent := (*C.char)(C.ostree_commit_get_parent(variant))
	defer C.free(unsafe.Pointer(parent))

	entries := make([]LogEntry, 0, 1)
	if parent != nil {
		var err error
		entries, err = logCommit(repo, parent, true, flags)
		if err != nil {
			return nil, err
		}
	}

	nextLogEntry := dumpLogObject(C.OSTREE_OBJECT_TYPE_COMMIT, checksum, variant, flags)
	entries = append(entries, nextLogEntry)

	return entries, nil
}

func dumpLogObject(objectType C.OstreeObjectType, checksum *C.char, variant *C.GVariant, flags ostreeDumpFlags) LogEntry {
	csum := []byte(C.GoString(checksum))

	if (flags & ostreeDumpRaw) != 0 {
		return dumpVariant(variant, csum)
	}

	switch objectType {
	case C.OSTREE_OBJECT_TYPE_COMMIT:
		return dumpCommit(variant, flags, csum)
	default:
		return LogEntry{
			Checksum: csum,
		}
	}
}

func dumpVariant(variant *C.GVariant, csum []byte) LogEntry {
	var logVariant []byte
	if C.G_BYTE_ORDER != C.G_BIG_ENDIAN {
		byteswappedVariant := C.g_variant_byteswap(variant)
		logVariant = []byte(C.GoString((*C.char)(C.g_variant_print(byteswappedVariant, C.TRUE))))
	} else {
		logVariant = []byte(C.GoString((*C.char)(C.g_variant_print(variant, C.TRUE))))
	}

	return LogEntry{
		Checksum: csum,
		Variant:  logVariant,
	}
}

func dumpCommit(variant *C.GVariant, flags ostreeDumpFlags, csum []byte) LogEntry {
	var subject *C.char
	defer C.free(unsafe.Pointer(subject))
	var body *C.char
	defer C.free(unsafe.Pointer(body))
	var timeBigE C.guint64

	C._g_variant_get_commit_dump(variant, C.CString("(a{sv}aya(say)&s&stayay)"), &subject, &body, &timeBigE)

	// Translate to a host-endian epoch and convert to Go timestamp
	timeHostE := C._guint64_from_be(timeBigE)
	timestamp := time.Unix((int64)(timeHostE), 0)

	return LogEntry{
		Timestamp: timestamp,
		Subject:   C.GoString(subject),
		Body:      C.GoString(body),
	}
}
