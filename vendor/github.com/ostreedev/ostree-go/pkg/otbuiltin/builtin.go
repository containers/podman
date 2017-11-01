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

type Repo struct {
	//*glib.GObject
	ptr unsafe.Pointer
}

// Converts an ostree repo struct to its C equivalent
func (r *Repo) native() *C.OstreeRepo {
	//return (*C.OstreeRepo)(r.Ptr())
	return (*C.OstreeRepo)(r.ptr)
}

// Takes a C ostree repo and converts it to a Go struct
func repoFromNative(p *C.OstreeRepo) *Repo {
	if p == nil {
		return nil
	}
	//o := (*glib.GObject)(unsafe.Pointer(p))
	//r := &Repo{o}
	r := &Repo{unsafe.Pointer(p)}
	return r
}

// Checks if the repo has been initialized
func (r *Repo) isInitialized() bool {
	if r.ptr != nil {
		return true
	}
	return false
}

// Attempts to open the repo at the given path
func OpenRepo(path string) (*Repo, error) {
	var cerr *C.GError = nil
	cpath := C.CString(path)
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	crepo := C.ostree_repo_new(pathc)
	repo := repoFromNative(crepo)
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_open(crepo, nil, &cerr)))
	if !r {
		return nil, generateError(cerr)
	}
	return repo, nil
}

// Enable support for tombstone commits, which allow the repo to distinguish between
// commits that were intentionally deleted and commits that were removed accidentally
func enableTombstoneCommits(repo *Repo) error {
	var tombstoneCommits bool
	var config *C.GKeyFile = C.ostree_repo_get_config(repo.native())
	var cerr *C.GError

	tombstoneCommits = glib.GoBool(glib.GBoolean(C.g_key_file_get_boolean(config, (*C.gchar)(C.CString("core")), (*C.gchar)(C.CString("tombstone-commits")), nil)))

	//tombstoneCommits is false only if it really is false or if it is set to FALSE in the config file
	if !tombstoneCommits {
		C.g_key_file_set_boolean(config, (*C.gchar)(C.CString("core")), (*C.gchar)(C.CString("tombstone-commits")), C.TRUE)
		if !glib.GoBool(glib.GBoolean(C.ostree_repo_write_config(repo.native(), config, &cerr))) {
			return generateError(cerr)
		}
	}
	return nil
}

func generateError(err *C.GError) error {
	goErr := glib.ConvertGError(glib.ToGError(unsafe.Pointer(err)))
	_, file, line, ok := runtime.Caller(1)
	if ok {
		return errors.New(fmt.Sprintf("%s:%d - %s", file, line, goErr))
	} else {
		return goErr
	}
}
