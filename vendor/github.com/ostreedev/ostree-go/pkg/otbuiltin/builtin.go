// Package otbuiltin contains all of the basic commands for creating and
// interacting with an ostree repository
package otbuiltin

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
)

// #cgo pkg-config: ostree-1
// #include <stdlib.h>
// #include <glib.h>
// #include <ostree.h>
// #include "builtin.go.h"
import "C"

// Repo represents a local ostree repository
type Repo struct {
	ptr unsafe.Pointer
}

// isInitialized checks if the repo has been initialized
func (r *Repo) isInitialized() bool {
	if r == nil || r.ptr == nil {
		return false
	}
	return true
}

// native converts an ostree repo struct to its C equivalent
func (r *Repo) native() *C.OstreeRepo {
	if !r.isInitialized() {
		return nil
	}
	return (*C.OstreeRepo)(r.ptr)
}

// repoFromNative takes a C ostree repo and converts it to a Go struct
func repoFromNative(or *C.OstreeRepo) *Repo {
	if or == nil {
		return nil
	}
	r := &Repo{unsafe.Pointer(or)}
	return r
}

// OpenRepo attempts to open the repo at the given path
func OpenRepo(path string) (*Repo, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	repoPath := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(repoPath))
	crepo := C.ostree_repo_new(repoPath)
	repo := repoFromNative(crepo)

	var cerr *C.GError
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_open(crepo, nil, &cerr)))
	if !r {
		return nil, generateError(cerr)
	}

	return repo, nil
}

// enableTombstoneCommits enables support for tombstone commits.
//
// This allows to distinguish between intentional deletions and accidental removals
// of commits.
func (r *Repo) enableTombstoneCommits() error {
	if !r.isInitialized() {
		return errors.New("repo not initialized")
	}

	config := C.ostree_repo_get_config(r.native())
	groupC := C.CString("core")
	defer C.free(unsafe.Pointer(groupC))
	keyC := C.CString("tombstone-commits")
	defer C.free(unsafe.Pointer(keyC))
	valueC := C.g_key_file_get_boolean(config, (*C.gchar)(groupC), (*C.gchar)(keyC), nil)
	tombstoneCommits := glib.GoBool(glib.GBoolean(valueC))

	// tombstoneCommits is false only if it really is false or if it is set to FALSE in the config file
	if !tombstoneCommits {
		var cerr *C.GError
		C.g_key_file_set_boolean(config, (*C.gchar)(groupC), (*C.gchar)(keyC), C.TRUE)
		if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_config(r.native(), config, &cerr))) {
			return generateError(cerr)
		}
	}
	return nil
}

// generateError wraps a GLib error into a Go one.
func generateError(err *C.GError) error {
	if err == nil {
		return errors.New("nil GError")
	}

	goErr := glib.ConvertGError(glib.ToGError(unsafe.Pointer(err)))
	_, file, line, ok := runtime.Caller(1)
	if ok {
		return fmt.Errorf("%s:%d - %s", file, line, goErr)
	}
	return goErr
}

// isOk wraps a gboolean return value into a bool.
// 0 is false/error, everything else is true/ok.
func isOk(v C.gboolean) bool {
	return glib.GoBool(glib.GBoolean(v))
}
