package otbuiltin

import (
	"errors"
	"unsafe"

	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
)

// #cgo pkg-config: ostree-1
// #include <stdlib.h>
// #include <glib.h>
// #include <ostree.h>
// #include "builtin.go.h"
import "C"

// checkoutOptions defines all of the options for checking commits
// out of an ostree repo
//
// Note: while this is private, fields are public and part of the API.
type checkoutOptions struct {
	// UserMode defines whether to checkout a repo in `bare-user` mode
	UserMode bool
	// Union specifies whether to overwrite existing filesystem entries
	Union bool
	// AllowNoEnt defines whether to skip filepaths that do not exist
	AllowNoent bool
	// DisableCache defines whether to disable internal repository uncompressed object cache
	DisableCache bool
	// Whiteouts defines whether to Process 'whiteout' (docker style) entries
	Whiteouts bool
	// RequireHardlinks defines whether to fall back to full copies if hard linking fails
	RequireHardlinks bool
	// SubPath specifies a sub-directory to use for checkout
	Subpath string
	// FromFile specifies an optional file containing many checkouts to process
	FromFile string
}

// NewCheckoutOptions instantiates and returns a checkoutOptions struct with default values set
func NewCheckoutOptions() checkoutOptions {
	return checkoutOptions{}
}

// Checkout checks out commit `commitRef` from a repository at `repoPath`,
// writing it to `destination`.  Returns an error if the checkout could not be processed.
func Checkout(repoPath, destination, commitRef string, opts checkoutOptions) error {
	var cancellable *glib.GCancellable

	ccommit := C.CString(commitRef)
	defer C.free(unsafe.Pointer(ccommit))

	var gerr = glib.NewGError()
	cerr := (*C.GError)(gerr.Ptr())
	defer C.free(unsafe.Pointer(cerr))

	repoPathc := C.g_file_new_for_path(C.CString(repoPath))
	defer C.g_object_unref(C.gpointer(repoPathc))
	crepo := C.ostree_repo_new(repoPathc)
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_open(crepo, (*C.GCancellable)(cancellable.Ptr()), &cerr))) {
		return generateError(cerr)
	}

	// Multiple checkouts to process
	if opts.FromFile != "" {
		return processManyCheckouts(crepo, destination, cancellable)
	}

	// Simple single checkout
	var resolvedCommit *C.char
	defer C.free(unsafe.Pointer(resolvedCommit))
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_resolve_rev(crepo, ccommit, C.FALSE, &resolvedCommit, &cerr))) {
		return generateError(cerr)
	}

	return processOneCheckout(crepo, resolvedCommit, destination, opts, cancellable)
}

// processOneCheckout processes one checkout from the repo
func processOneCheckout(crepo *C.OstreeRepo, resolvedCommit *C.char, destination string, opts checkoutOptions, cancellable *glib.GCancellable) error {
	cdest := C.CString(destination)
	defer C.free(unsafe.Pointer(cdest))

	var gerr = glib.NewGError()
	cerr := (*C.GError)(gerr.Ptr())
	defer C.free(unsafe.Pointer(cerr))

	// Process options into bitflags
	var repoCheckoutAtOptions C.OstreeRepoCheckoutAtOptions
	if opts.UserMode {
		repoCheckoutAtOptions.mode = C.OSTREE_REPO_CHECKOUT_MODE_USER
	}
	if opts.Union {
		repoCheckoutAtOptions.overwrite_mode = C.OSTREE_REPO_CHECKOUT_OVERWRITE_UNION_FILES
	}
	if opts.RequireHardlinks {
		repoCheckoutAtOptions.no_copy_fallback = C.TRUE
	}

	// Checkout commit to destination
	if !glib.GoBool(glib.GBoolean(C.ostree_repo_checkout_at(crepo, &repoCheckoutAtOptions, C._at_fdcwd(), cdest, resolvedCommit, nil, &cerr))) {
		return generateError(cerr)
	}

	return nil
}

// processManyCheckouts processes many checkouts in a single batch
func processManyCheckouts(crepo *C.OstreeRepo, target string, cancellable *glib.GCancellable) error {
	return errors.New("batch checkouts processing: not implemented")
}
