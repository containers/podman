package otbuiltin

import (
	"bytes"
	"errors"
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

// Declare global variable to store commitOptions
var options commitOptions

// Declare a function prototype for being passed into another function
type handleLineFunc func(string, *glib.GHashTable) error

// Contains all of the options for commmiting to an ostree repo.  Initialize
// with NewCommitOptions()
type commitOptions struct {
	Subject                   string    // One line subject
	Body                      string    // Full description
	Parent                    string    // Parent of the commit
	Tree                      []string  // 'dir=PATH' or 'tar=TARFILE' or 'ref=COMMIT': overlay the given argument as a tree
	AddMetadataString         []string  // Add a key/value pair to metadata
	AddDetachedMetadataString []string  // Add a key/value pair to detached metadata
	OwnerUID                  int       // Set file ownership to user id
	OwnerGID                  int       // Set file ownership to group id
	NoXattrs                  bool      // Do not import extended attributes
	LinkCheckoutSpeedup       bool      // Optimize for commits of trees composed of hardlinks in the repository
	TarAutoCreateParents      bool      // When loading tar archives, automatically create parent directories as needed
	SkipIfUnchanged           bool      // If the contents are unchanged from a previous commit, do nothing
	StatOverrideFile          string    // File containing list of modifications to make permissions
	SkipListFile              string    // File containing list of file paths to skip
	GenerateSizes             bool      // Generate size information along with commit metadata
	GpgSign                   []string  // GPG Key ID with which to sign the commit (if you have GPGME - GNU Privacy Guard Made Easy)
	GpgHomedir                string    // GPG home directory to use when looking for keyrings (if you have GPGME - GNU Privacy Guard Made Easy)
	Timestamp                 time.Time // Override the timestamp of the commit
	Orphan                    bool      // Commit does not belong to a branch
	Fsync                     bool      // Specify whether fsync should be used or not.  Default to true
}

// Initializes a commitOptions struct and sets default values
func NewCommitOptions() commitOptions {
	var co commitOptions
	co.OwnerUID = -1
	co.OwnerGID = -1
	co.Fsync = true
	return co
}

type OstreeRepoTransactionStats struct {
	metadata_objects_total   int32
	metadata_objects_written int32
	content_objects_total    int32
	content_objects_written  int32
	content_bytes_written    uint64
}

func (repo *Repo) PrepareTransaction() (bool, error) {
	var cerr *C.GError = nil
	var resume C.gboolean

	r := glib.GoBool(glib.GBoolean(C.ostree_repo_prepare_transaction(repo.native(), &resume, nil, &cerr)))
	if !r {
		return false, generateError(cerr)
	}
	return glib.GoBool(glib.GBoolean(resume)), nil
}

func (repo *Repo) CommitTransaction() (*OstreeRepoTransactionStats, error) {
	var cerr *C.GError = nil
	var stats OstreeRepoTransactionStats = OstreeRepoTransactionStats{}
	statsPtr := (*C.OstreeRepoTransactionStats)(unsafe.Pointer(&stats))
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_commit_transaction(repo.native(), statsPtr, nil, &cerr)))
	if !r {
		return nil, generateError(cerr)
	}
	return &stats, nil
}

func (repo *Repo) TransactionSetRef(remote string, ref string, checksum string) {
	var cRemote *C.char = nil
	var cRef *C.char = nil
	var cChecksum *C.char = nil

	if remote != "" {
		cRemote = C.CString(remote)
	}
	if ref != "" {
		cRef = C.CString(ref)
	}
	if checksum != "" {
		cChecksum = C.CString(checksum)
	}
	C.ostree_repo_transaction_set_ref(repo.native(), cRemote, cRef, cChecksum)
}

func (repo *Repo) AbortTransaction() error {
	var cerr *C.GError = nil
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_abort_transaction(repo.native(), nil, &cerr)))
	if !r {
		return generateError(cerr)
	}
	return nil
}

func (repo *Repo) RegenerateSummary() error {
	var cerr *C.GError = nil
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_regenerate_summary(repo.native(), nil, nil, &cerr)))
	if !r {
		return generateError(cerr)
	}
	return nil
}

// Commits a directory, specified by commitPath, to an ostree repo as a given branch
func (repo *Repo) Commit(commitPath, branch string, opts commitOptions) (string, error) {
	// TODO(lucab): `options` is global un-synchronized mutable state, get rid of it.
	options = opts

	var err error
	var modeAdds *glib.GHashTable
	var skipList *glib.GHashTable
	var objectToCommit *glib.GFile
	var skipCommit bool = false
	var ccommitChecksum *C.char
	defer C.free(unsafe.Pointer(ccommitChecksum))
	var flags C.OstreeRepoCommitModifierFlags = 0
	var filter_data C.CommitFilterData

	var cerr *C.GError
	defer C.free(unsafe.Pointer(cerr))
	var metadata *C.GVariant = nil
	defer func() {
		if metadata != nil {
			defer C.g_variant_unref(metadata)
		}
	}()

	var detachedMetadata *C.GVariant = nil
	defer C.free(unsafe.Pointer(detachedMetadata))
	var mtree *C.OstreeMutableTree
	defer C.free(unsafe.Pointer(mtree))
	var root *C.GFile
	defer C.free(unsafe.Pointer(root))
	var modifier *C.OstreeRepoCommitModifier
	defer C.free(unsafe.Pointer(modifier))
	var cancellable *C.GCancellable
	defer C.free(unsafe.Pointer(cancellable))

	cpath := C.CString(commitPath)
	defer C.free(unsafe.Pointer(cpath))
	csubject := C.CString(options.Subject)
	defer C.free(unsafe.Pointer(csubject))
	cbody := C.CString(options.Body)
	defer C.free(unsafe.Pointer(cbody))
	cbranch := C.CString(branch)
	defer C.free(unsafe.Pointer(cbranch))
	cparent := C.CString(options.Parent)
	defer C.free(unsafe.Pointer(cparent))

	if !glib.GoBool(glib.GBoolean(C.ostree_repo_is_writable(repo.native(), &cerr))) {
		goto out
	}

	// If the user provided a stat override file
	if strings.Compare(options.StatOverrideFile, "") != 0 {
		modeAdds = glib.ToGHashTable(unsafe.Pointer(C._g_hash_table_new_full()))
		if err = parseFileByLine(options.StatOverrideFile, handleStatOverrideLine, modeAdds, cancellable); err != nil {
			goto out
		}
	}

	// If the user provided a skiplist file
	if strings.Compare(options.SkipListFile, "") != 0 {
		skipList = glib.ToGHashTable(unsafe.Pointer(C._g_hash_table_new_full()))
		if err = parseFileByLine(options.SkipListFile, handleSkipListline, skipList, cancellable); err != nil {
			goto out
		}
	}

	if options.AddMetadataString != nil {
		metadata, err = parseKeyValueStrings(options.AddMetadataString)
		if err != nil {
			goto out
		}
	}

	if options.AddDetachedMetadataString != nil {
		_, err = parseKeyValueStrings(options.AddDetachedMetadataString)
		if err != nil {
			goto out
		}
	}

	if strings.Compare(branch, "") == 0 && !options.Orphan {
		err = errors.New("A branch must be specified or use commitOptions.Orphan")
		goto out
	}

	if options.NoXattrs {
		C._ostree_repo_append_modifier_flags(&flags, C.OSTREE_REPO_COMMIT_MODIFIER_FLAGS_SKIP_XATTRS)
	}
	if options.GenerateSizes {
		C._ostree_repo_append_modifier_flags(&flags, C.OSTREE_REPO_COMMIT_MODIFIER_FLAGS_GENERATE_SIZES)
	}
	if !options.Fsync {
		C.ostree_repo_set_disable_fsync(repo.native(), C.TRUE)
	}

	if flags != 0 || options.OwnerUID >= 0 || options.OwnerGID >= 0 || strings.Compare(options.StatOverrideFile, "") != 0 || options.NoXattrs {
		filter_data.mode_adds = (*C.GHashTable)(modeAdds.Ptr())
		filter_data.skip_list = (*C.GHashTable)(skipList.Ptr())
		C._set_owner_uid((C.guint32)(options.OwnerUID))
		C._set_owner_gid((C.guint32)(options.OwnerGID))
		modifier = C._ostree_repo_commit_modifier_new_wrapper(flags, C.gpointer(&filter_data), nil)
	}

	if strings.Compare(options.Parent, "") != 0 {
		if strings.Compare(options.Parent, "none") == 0 {
			options.Parent = ""
		}
	} else if !options.Orphan {
		cerr = nil
		if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(repo.native(), cbranch, C.TRUE, &cparent, &cerr))) {
			goto out
		}
	}

	if options.LinkCheckoutSpeedup && !glib.GoBool(glib.GBoolean(C.ostree_repo_scan_hardlinks(repo.native(), cancellable, &cerr))) {
		goto out
	}

	mtree = C.ostree_mutable_tree_new()

	if len(commitPath) == 0 && (len(options.Tree) == 0 || len(options.Tree[0]) == 0) {
		currentDir := (*C.char)(C.g_get_current_dir())
		objectToCommit = glib.ToGFile(unsafe.Pointer(C.g_file_new_for_path(currentDir)))
		C.g_free(C.gpointer(currentDir))

		if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_directory_to_mtree(repo.native(), (*C.GFile)(objectToCommit.Ptr()), mtree, modifier, cancellable, &cerr))) {
			goto out
		}
	} else if len(options.Tree) != 0 {
		var eq int = -1
		cerr = nil
		for tree := range options.Tree {
			eq = strings.Index(options.Tree[tree], "=")
			if eq == -1 {
				C._g_set_error_onearg(cerr, C.CString("Missing type in tree specification"), C.CString(options.Tree[tree]))
				goto out
			}
			treeType := options.Tree[tree][:eq]
			treeVal := options.Tree[tree][eq+1:]

			if strings.Compare(treeType, "dir") == 0 {
				objectToCommit = glib.ToGFile(unsafe.Pointer(C.g_file_new_for_path(C.CString(treeVal))))
				if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_directory_to_mtree(repo.native(), (*C.GFile)(objectToCommit.Ptr()), mtree, modifier, cancellable, &cerr))) {
					goto out
				}
			} else if strings.Compare(treeType, "tar") == 0 {
				objectToCommit = glib.ToGFile(unsafe.Pointer(C.g_file_new_for_path(C.CString(treeVal))))
				if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_archive_to_mtree(repo.native(), (*C.GFile)(objectToCommit.Ptr()), mtree, modifier, (C.gboolean)(glib.GBool(opts.TarAutoCreateParents)), cancellable, &cerr))) {
					fmt.Println("error 1")
					goto out
				}
			} else if strings.Compare(treeType, "ref") == 0 {
				if !glib.GoBool(glib.GBoolean(C.ostree_repo_read_commit(repo.native(), C.CString(treeVal), (**C.GFile)(objectToCommit.Ptr()), nil, cancellable, &cerr))) {
					goto out
				}

				if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_directory_to_mtree(repo.native(), (*C.GFile)(objectToCommit.Ptr()), mtree, modifier, cancellable, &cerr))) {
					goto out
				}
			} else {
				C._g_set_error_onearg(cerr, C.CString("Missing type in tree specification"), C.CString(treeVal))
				goto out
			}
		}
	} else {
		objectToCommit = glib.ToGFile(unsafe.Pointer(C.g_file_new_for_path(cpath)))
		cerr = nil
		if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_directory_to_mtree(repo.native(), (*C.GFile)(objectToCommit.Ptr()), mtree, modifier, cancellable, &cerr))) {
			goto out
		}
	}

	if modeAdds != nil && C.g_hash_table_size((*C.GHashTable)(modeAdds.Ptr())) > 0 {
		var hashIter *C.GHashTableIter

		var key, value C.gpointer

		C.g_hash_table_iter_init(hashIter, (*C.GHashTable)(modeAdds.Ptr()))

		for glib.GoBool(glib.GBoolean(C.g_hash_table_iter_next(hashIter, &key, &value))) {
			C._g_printerr_onearg(C.CString("Unmatched StatOverride path: "), C._gptr_to_str(key))
		}
		err = errors.New("Unmatched StatOverride paths")
		C.free(unsafe.Pointer(hashIter))
		C.free(unsafe.Pointer(key))
		C.free(unsafe.Pointer(value))
		goto out
	}

	if skipList != nil && C.g_hash_table_size((*C.GHashTable)(skipList.Ptr())) > 0 {
		var hashIter *C.GHashTableIter
		var key, value C.gpointer

		C.g_hash_table_iter_init(hashIter, (*C.GHashTable)(skipList.Ptr()))

		for glib.GoBool(glib.GBoolean(C.g_hash_table_iter_next(hashIter, &key, &value))) {
			C._g_printerr_onearg(C.CString("Unmatched SkipList path: "), C._gptr_to_str(key))
		}
		err = errors.New("Unmatched SkipList paths")
		C.free(unsafe.Pointer(hashIter))
		C.free(unsafe.Pointer(key))
		C.free(unsafe.Pointer(value))
		goto out
	}

	cerr = nil
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_mtree(repo.native(), mtree, &root, cancellable, &cerr))) {
		goto out
	}

	if options.SkipIfUnchanged && strings.Compare(options.Parent, "") != 0 {
		var parentRoot *C.GFile

		cerr = nil
		if !glib.GoBool(glib.GBoolean(C.ostree_repo_read_commit(repo.native(), cparent, &parentRoot, nil, cancellable, &cerr))) {
			C.free(unsafe.Pointer(parentRoot))
			goto out
		}

		if glib.GoBool(glib.GBoolean(C.g_file_equal(root, parentRoot))) {
			skipCommit = true
		}
		C.free(unsafe.Pointer(parentRoot))
	}

	if !skipCommit {
		var timestamp C.guint64

		if options.Timestamp.IsZero() {
			var now *C.GDateTime = C.g_date_time_new_now_utc()
			timestamp = (C.guint64)(C.g_date_time_to_unix(now))
			C.g_date_time_unref(now)

			cerr = nil
			ret := C.ostree_repo_write_commit(repo.native(), cparent, csubject, cbody, metadata, C._ostree_repo_file(root), &ccommitChecksum, cancellable, &cerr)
			if !glib.GoBool(glib.GBoolean(ret)) {
				goto out
			}
		} else {
			timestamp = (C.guint64)(options.Timestamp.Unix())

			if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_commit_with_time(repo.native(), cparent, csubject, cbody,
				metadata, C._ostree_repo_file(root), timestamp, &ccommitChecksum, cancellable, &cerr))) {
				goto out
			}
		}

		if detachedMetadata != nil {
			C.ostree_repo_write_commit_detached_metadata(repo.native(), ccommitChecksum, detachedMetadata, cancellable, &cerr)
		}

		if len(options.GpgSign) != 0 {
			for key := range options.GpgSign {
				if !glib.GoBool(glib.GBoolean(C.ostree_repo_sign_commit(repo.native(), (*C.gchar)(ccommitChecksum), (*C.gchar)(C.CString(options.GpgSign[key])), (*C.gchar)(C.CString(options.GpgHomedir)), cancellable, &cerr))) {
					goto out
				}
			}
		}

		if strings.Compare(branch, "") != 0 {
			C.ostree_repo_transaction_set_ref(repo.native(), nil, cbranch, ccommitChecksum)
		} else if !options.Orphan {
			goto out
		} else {
			// TODO: Looks like I forgot to implement this.
		}
	} else {
		ccommitChecksum = C.CString(options.Parent)
	}

	return C.GoString(ccommitChecksum), nil
out:
	if repo.native() != nil {
		C.ostree_repo_abort_transaction(repo.native(), cancellable, nil)
		//C.free(unsafe.Pointer(repo.native()))
	}
	if modifier != nil {
		C.ostree_repo_commit_modifier_unref(modifier)
	}
	if err != nil {
		return "", err
	}
	return "", generateError(cerr)
}

// Parse an array of key value pairs of the format KEY=VALUE and add them to a GVariant
func parseKeyValueStrings(pairs []string) (*C.GVariant, error) {
	builder := C.g_variant_builder_new(C._g_variant_type(C.CString("a{sv}")))
	defer C.g_variant_builder_unref(builder)

	for iter := range pairs {
		index := strings.Index(pairs[iter], "=")
		if index <= 0 {
			var buffer bytes.Buffer
			buffer.WriteString("Missing '=' in KEY=VALUE metadata '%s'")
			buffer.WriteString(pairs[iter])
			return nil, errors.New(buffer.String())
		}

		key := C.CString(pairs[iter][:index])
		value := C.CString(pairs[iter][index+1:])

		valueVariant := C.g_variant_new_string((*C.gchar)(value))

		C._g_variant_builder_add_twoargs(builder, C.CString("{sv}"), key, valueVariant)
	}

	metadata := C.g_variant_builder_end(builder)
	return C.g_variant_ref_sink(metadata), nil
}

// Parse a file linue by line and handle the line with the handleLineFunc
func parseFileByLine(path string, fn handleLineFunc, table *glib.GHashTable, cancellable *C.GCancellable) error {
	var contents *C.char
	var file *glib.GFile
	var lines []string
	var gerr = glib.NewGError()
	cerr := (*C.GError)(gerr.Ptr())

	file = glib.ToGFile(unsafe.Pointer(C.g_file_new_for_path(C.CString(path))))
	if !glib.GoBool(glib.GBoolean(C.g_file_load_contents((*C.GFile)(file.Ptr()), cancellable, &contents, nil, nil, &cerr))) {
		return generateError(cerr)
	}

	lines = strings.Split(C.GoString(contents), "\n")
	for line := range lines {
		if strings.Compare(lines[line], "") == 0 {
			continue
		}

		if err := fn(lines[line], table); err != nil {
			return generateError(cerr)
		}
	}
	return nil
}

// Handle an individual line from a Statoverride file
func handleStatOverrideLine(line string, table *glib.GHashTable) error {
	var space int
	var modeAdd C.guint

	if space = strings.IndexRune(line, ' '); space == -1 {
		return errors.New("Malformed StatOverrideFile (no space found)")
	}

	modeAdd = (C.guint)(C.g_ascii_strtod((*C.gchar)(C.CString(line)), nil))
	C.g_hash_table_insert((*C.GHashTable)(table.Ptr()), C.gpointer(C.g_strdup((*C.gchar)(C.CString(line[space+1:])))), C._guint_to_pointer(modeAdd))

	return nil
}

// Handle an individual line from a Skiplist file
func handleSkipListline(line string, table *glib.GHashTable) error {
	C.g_hash_table_add((*C.GHashTable)(table.Ptr()), C.gpointer(C.g_strdup((*C.gchar)(C.CString(line)))))

	return nil
}
