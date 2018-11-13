package otbuiltin

import (
	"strings"
	"unsafe"
)

// #cgo pkg-config: ostree-1
// #include <stdlib.h>
// #include <glib.h>
// #include <ostree.h>
// #include "builtin.go.h"
import "C"

// initOptions contains all of the options for initializing an ostree repo
//
// Note: while this is private, exported fields are public and part of the API.
type initOptions struct {
	// Mode defines repository mode: either bare, archive-z2, or bare-user
	Mode string
}

// NewInitOptions instantiates and returns an initOptions struct with default values set
func NewInitOptions() initOptions {
	return initOptions{
		Mode: "bare",
	}
}

// Init initializes a new ostree repository at the given path.  Returns true
// if the repo exists at the location, regardless of whether it was initialized
// by the function or if it already existed.  Returns an error if the repo could
// not be initialized
func Init(path string, options initOptions) (bool, error) {
	repoMode, err := parseRepoMode(options.Mode)
	if err != nil {
		return false, err
	}

	// Create a repo struct from the path
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	repo := C.ostree_repo_new(pathc)

	// If the repo exists in the filesystem, return an error but set exists to true
	/* var exists C.gboolean = 0
	success := glib.GoBool(glib.GBoolean(C.ostree_repo_exists(crepo, &exists, &cerr)))
	if exists != 0 {
	  err = errors.New("repository already exists")
	  return true, err
	} else if !success {
	  return false, generateError(cerr)
	}*/

	var cErr *C.GError
	defer C.free(unsafe.Pointer(cErr))
	if r := C.ostree_repo_create(repo, repoMode, nil, &cErr); !isOk(r) {
		err := generateError(cErr)
		if strings.Contains(err.Error(), "File exists") {
			return true, err
		}
		return false, err
	}
	return true, nil
}

// parseRepoMode converts a mode string to a C.OSTREE_REPO_MODE enum value
func parseRepoMode(modeLabel string) (C.OstreeRepoMode, error) {
	var cErr *C.GError
	defer C.free(unsafe.Pointer(cErr))

	cModeLabel := C.CString(modeLabel)
	defer C.free(unsafe.Pointer(cModeLabel))

	var retMode C.OstreeRepoMode
	if r := C.ostree_repo_mode_from_string(cModeLabel, &retMode, &cErr); !isOk(r) {
		// NOTE(lucab): zero-value for this C enum has no special/invalid meaning.
		return C.OSTREE_REPO_MODE_BARE, generateError(cErr)
	}

	return retMode, nil
}
