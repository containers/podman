package otbuiltin

import (
	"errors"
	"strings"
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
var initOpts initOptions

// Contains all of the options for initializing an ostree repo
type initOptions struct {
	Mode string // either bare, archive-z2, or bare-user

	repoMode C.OstreeRepoMode
}

// Instantiates and returns an initOptions struct with default values set
func NewInitOptions() initOptions {
	io := initOptions{}
	io.Mode = "bare"
	io.repoMode = C.OSTREE_REPO_MODE_BARE
	return io
}

// Initializes a new ostree repository at the given path.  Returns true
// if the repo exists at the location, regardless of whether it was initialized
// by the function or if it already existed.  Returns an error if the repo could
// not be initialized
func Init(path string, options initOptions) (bool, error) {
	initOpts = options
	err := parseMode()
	if err != nil {
		return false, err
	}

	// Create a repo struct from the path
	var cerr *C.GError
	defer C.free(unsafe.Pointer(cerr))
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	crepo := C.ostree_repo_new(pathc)

	// If the repo exists in the filesystem, return an error but set exists to true
	/* var exists C.gboolean = 0
	success := glib.GoBool(glib.GBoolean(C.ostree_repo_exists(crepo, &exists, &cerr)))
	if exists != 0 {
	  err = errors.New("repository already exists")
	  return true, err
	} else if !success {
	  return false, generateError(cerr)
	}*/

	cerr = nil
	created := glib.GoBool(glib.GBoolean(C.ostree_repo_create(crepo, initOpts.repoMode, nil, &cerr)))
	if !created {
		errString := generateError(cerr).Error()
		if strings.Contains(errString, "File exists") {
			return true, generateError(cerr)
		}
		return false, generateError(cerr)
	}
	return true, nil
}

// Converts the mode string to a C.OSTREE_REPO_MODE enum value
func parseMode() error {
	if strings.EqualFold(initOpts.Mode, "bare") {
		initOpts.repoMode = C.OSTREE_REPO_MODE_BARE
	} else if strings.EqualFold(initOpts.Mode, "bare-user") {
		initOpts.repoMode = C.OSTREE_REPO_MODE_BARE_USER
	} else if strings.EqualFold(initOpts.Mode, "archive-z2") {
		initOpts.repoMode = C.OSTREE_REPO_MODE_ARCHIVE_Z2
	} else {
		return errors.New("Invalid option for mode")
	}
	return nil
}
