package otbuiltin

import (
	"fmt"
	"strings"
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

// Declare variables for options
var logOpts logOptions

// Set the format of the strings in the log
const formatString = "2006-01-02 03:04;05 -0700"

// Struct for the various pieces of data in a log entry
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

type OstreeDumpFlags uint

const (
	OSTREE_DUMP_NONE OstreeDumpFlags = 0
	OSTREE_DUMP_RAW  OstreeDumpFlags = 1 << iota
)

// Contains all of the options for initializing an ostree repo
type logOptions struct {
	Raw bool // Show raw variant data
}

//Instantiates and returns a logOptions struct with default values set
func NewLogOptions() logOptions {
	return logOptions{}
}

// Show the logs of a branch starting with a given commit or ref.  Returns a
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
	var flags OstreeDumpFlags = OSTREE_DUMP_NONE
	var cerr *C.GError
	defer C.free(unsafe.Pointer(cerr))

	if logOpts.Raw {
		flags |= OSTREE_DUMP_RAW
	}

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.native(), cbranch, C.FALSE, &checksum, &cerr))) {
		return nil, generateError(cerr)
	}

	return logCommit(repo, checksum, false, flags)
}

func logCommit(repo *Repo, checksum *C.char, isRecursive bool, flags OstreeDumpFlags) ([]LogEntry, error) {
	var variant *C.GVariant
	var parent *C.char
	defer C.free(unsafe.Pointer(parent))
	var gerr = glib.NewGError()
	var cerr = (*C.GError)(gerr.Ptr())
	defer C.free(unsafe.Pointer(cerr))
	entries := make([]LogEntry, 0, 1)
	var err error

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_load_variant(repo.native(), C.OSTREE_OBJECT_TYPE_COMMIT, checksum, &variant, &cerr))) {
		if isRecursive && glib.GoBool(glib.GBoolean(C.g_error_matches(cerr, C.g_io_error_quark(), C.G_IO_ERROR_NOT_FOUND))) {
			return nil, nil
		}
		return entries, generateError(cerr)
	}

	nextLogEntry := dumpLogObject(C.OSTREE_OBJECT_TYPE_COMMIT, checksum, variant, flags)

	// get the parent of this commit
	parent = (*C.char)(C.ostree_commit_get_parent(variant))
	defer C.free(unsafe.Pointer(parent))
	if parent != nil {
		entries, err = logCommit(repo, parent, true, flags)
		if err != nil {
			return nil, err
		}
	}
	entries = append(entries, *nextLogEntry)
	return entries, nil
}

func dumpLogObject(objectType C.OstreeObjectType, checksum *C.char, variant *C.GVariant, flags OstreeDumpFlags) *LogEntry {
	objLog := new(LogEntry)
	objLog.Checksum = []byte(C.GoString(checksum))

	if (flags & OSTREE_DUMP_RAW) != 0 {
		dumpVariant(objLog, variant)
		return objLog
	}

	switch objectType {
	case C.OSTREE_OBJECT_TYPE_COMMIT:
		dumpCommit(objLog, variant, flags)
		return objLog
	default:
		return objLog
	}
}

func dumpVariant(log *LogEntry, variant *C.GVariant) {
	var byteswappedVariant *C.GVariant

	if C.G_BYTE_ORDER != C.G_BIG_ENDIAN {
		byteswappedVariant = C.g_variant_byteswap(variant)
		log.Variant = []byte(C.GoString((*C.char)(C.g_variant_print(byteswappedVariant, C.TRUE))))
	} else {
		log.Variant = []byte(C.GoString((*C.char)(C.g_variant_print(byteswappedVariant, C.TRUE))))
	}
}

func dumpCommit(log *LogEntry, variant *C.GVariant, flags OstreeDumpFlags) {
	var subject, body *C.char
	defer C.free(unsafe.Pointer(subject))
	defer C.free(unsafe.Pointer(body))
	var timestamp C.guint64

	C._g_variant_get_commit_dump(variant, C.CString("(a{sv}aya(say)&s&stayay)"), &subject, &body, &timestamp)

	// Timestamp is now a Unix formatted timestamp as a guint64
	timestamp = C._guint64_from_be(timestamp)
	log.Timestamp = time.Unix((int64)(timestamp), 0)

	if strings.Compare(C.GoString(subject), "") != 0 {
		log.Subject = C.GoString(subject)
	}

	if strings.Compare(C.GoString(body), "") != 0 {
		log.Body = C.GoString(body)
	}
}
