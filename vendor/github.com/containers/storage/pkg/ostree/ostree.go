// +build ostree,cgo

package ostree

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	glib "github.com/ostreedev/ostree-go/pkg/glibobject"
	"github.com/ostreedev/ostree-go/pkg/otbuiltin"
	"github.com/pkg/errors"
)

// #cgo pkg-config: glib-2.0 gobject-2.0 ostree-1
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <stdlib.h>
// #include <ostree.h>
// #include <gio/ginputstream.h>
import "C"

func OstreeSupport() bool {
	return true
}

func fixFiles(dir string, usermode bool) (bool, []string, error) {
	var SkipOstree = errors.New("skip ostree deduplication")

	var whiteouts []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.Mode()&(os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
			if !usermode {
				stat, ok := info.Sys().(*syscall.Stat_t)
				if !ok {
					return errors.New("not syscall.Stat_t")
				}

				if stat.Rdev == 0 && (stat.Mode&unix.S_IFCHR) != 0 {
					whiteouts = append(whiteouts, path)
					return nil
				}
			}
			// Skip the ostree deduplication if we encounter a file type that
			// ostree does not manage.
			return SkipOstree
		}
		if info.IsDir() {
			if usermode {
				if err := os.Chmod(path, info.Mode()|0700); err != nil {
					return err
				}
			}
		} else if usermode && (info.Mode().IsRegular()) {
			if err := os.Chmod(path, info.Mode()|0600); err != nil {
				return err
			}
		}
		return nil
	})
	if err == SkipOstree {
		return true, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return false, whiteouts, nil
}

// Create prepares the filesystem for the OSTREE driver and copies the directory for the given id under the parent.
func ConvertToOSTree(repoLocation, root, id string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	repo, err := otbuiltin.OpenRepo(repoLocation)
	if err != nil {
		return errors.Wrap(err, "could not open the OSTree repository")
	}

	skip, whiteouts, err := fixFiles(root, os.Getuid() != 0)
	if err != nil {
		return errors.Wrap(err, "could not prepare the OSTree directory")
	}
	if skip {
		return nil
	}

	if _, err := repo.PrepareTransaction(); err != nil {
		return errors.Wrap(err, "could not prepare the OSTree transaction")
	}

	if skip {
		return nil
	}

	commitOpts := otbuiltin.NewCommitOptions()
	commitOpts.Timestamp = time.Now()
	commitOpts.LinkCheckoutSpeedup = true
	commitOpts.Parent = "0000000000000000000000000000000000000000000000000000000000000000"
	branch := fmt.Sprintf("containers-storage/%s", id)

	for _, w := range whiteouts {
		if err := os.Remove(w); err != nil {
			return errors.Wrap(err, "could not delete whiteout file")
		}
	}

	if _, err := repo.Commit(root, branch, commitOpts); err != nil {
		return errors.Wrap(err, "could not commit the layer")
	}

	if _, err := repo.CommitTransaction(); err != nil {
		return errors.Wrap(err, "could not complete the OSTree transaction")
	}

	if err := system.EnsureRemoveAll(root); err != nil {
		return errors.Wrap(err, "could not delete layer")
	}

	checkoutOpts := otbuiltin.NewCheckoutOptions()
	checkoutOpts.RequireHardlinks = true
	checkoutOpts.Whiteouts = false
	if err := otbuiltin.Checkout(repoLocation, root, branch, checkoutOpts); err != nil {
		return errors.Wrap(err, "could not checkout from OSTree")
	}

	for _, w := range whiteouts {
		if err := unix.Mknod(w, unix.S_IFCHR, 0); err != nil {
			return errors.Wrap(err, "could not recreate whiteout file")
		}
	}
	return nil
}

func CreateOSTreeRepository(repoLocation string, rootUID int, rootGID int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, err := os.Stat(repoLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil {
		if err := idtools.MkdirAllAs(repoLocation, 0700, rootUID, rootGID); err != nil {
			return errors.Wrap(err, "could not create OSTree repository directory: %v")
		}

		if _, err := otbuiltin.Init(repoLocation, otbuiltin.NewInitOptions()); err != nil {
			return errors.Wrap(err, "could not create OSTree repository")
		}
	}
	return nil
}

func openRepo(path string) (*C.struct_OstreeRepo, error) {
	var cerr *C.GError
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	pathc := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(pathc))
	repo := C.ostree_repo_new(pathc)
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_open(repo, nil, &cerr)))
	if !r {
		C.g_object_unref(C.gpointer(repo))
		return nil, glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return repo, nil
}

func DeleteOSTree(repoLocation, id string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	repo, err := openRepo(repoLocation)
	if err != nil {
		return err
	}
	defer C.g_object_unref(C.gpointer(repo))

	branch := fmt.Sprintf("containers-storage/%s", id)

	cbranch := C.CString(branch)
	defer C.free(unsafe.Pointer(cbranch))

	var cerr *C.GError
	r := glib.GoBool(glib.GBoolean(C.ostree_repo_set_ref_immediate(repo, nil, cbranch, nil, nil, &cerr)))
	if !r {
		return glib.ConvertGError(glib.ToGError(unsafe.Pointer(cerr)))
	}
	return nil
}
