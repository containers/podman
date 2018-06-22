package buildah

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah/util"
	"github.com/projectatomic/libpod/pkg/chrootuser"
	"github.com/sirupsen/logrus"
)

// AddAndCopyOptions holds options for add and copy commands.
type AddAndCopyOptions struct {
	// Chown is a spec for the user who should be given ownership over the
	// newly-added content, potentially overriding permissions which would
	// otherwise match those of local files and directories being copied.
	Chown string
	// All of the data being copied will pass through Hasher, if set.
	// If the sources are URLs or files, their contents will be passed to
	// Hasher.
	// If the sources include directory trees, Hasher will be passed
	// tar-format archives of the directory trees.
	Hasher io.Writer
}

// addURL copies the contents of the source URL to the destination.  This is
// its own function so that deferred closes happen after we're done pulling
// down each item of potentially many.
func addURL(destination, srcurl string, owner idtools.IDPair, hasher io.Writer) error {
	logrus.Debugf("saving %q to %q", srcurl, destination)
	resp, err := http.Get(srcurl)
	if err != nil {
		return errors.Wrapf(err, "error getting %q", srcurl)
	}
	defer resp.Body.Close()
	f, err := os.Create(destination)
	if err != nil {
		return errors.Wrapf(err, "error creating %q", destination)
	}
	if err = f.Chown(owner.UID, owner.GID); err != nil {
		return errors.Wrapf(err, "error setting owner of %q", destination)
	}
	if last := resp.Header.Get("Last-Modified"); last != "" {
		if mtime, err2 := time.Parse(time.RFC1123, last); err2 != nil {
			logrus.Debugf("error parsing Last-Modified time %q: %v", last, err2)
		} else {
			defer func() {
				if err3 := os.Chtimes(destination, time.Now(), mtime); err3 != nil {
					logrus.Debugf("error setting mtime to Last-Modified time %q: %v", last, err3)
				}
			}()
		}
	}
	defer f.Close()
	bodyReader := io.Reader(resp.Body)
	if hasher != nil {
		bodyReader = io.TeeReader(bodyReader, hasher)
	}
	n, err := io.Copy(f, bodyReader)
	if err != nil {
		return errors.Wrapf(err, "error reading contents for %q", destination)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return errors.Errorf("error reading contents for %q: wrong length (%d != %d)", destination, n, resp.ContentLength)
	}
	if err := f.Chmod(0600); err != nil {
		return errors.Wrapf(err, "error setting permissions on %q", destination)
	}
	return nil
}

// Add copies the contents of the specified sources into the container's root
// filesystem, optionally extracting contents of local files that look like
// non-empty archives.
func (b *Builder) Add(destination string, extract bool, options AddAndCopyOptions, source ...string) error {
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := b.Unmount(); err2 != nil {
			logrus.Errorf("error unmounting container: %v", err2)
		}
	}()
	// Find out which user (and group) the destination should belong to.
	user, err := b.user(mountPoint, options.Chown)
	if err != nil {
		return err
	}
	containerOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}
	hostUID, hostGID, err := util.GetHostIDs(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap, user.UID, user.GID)
	if err != nil {
		return err
	}
	hostOwner := idtools.IDPair{UID: int(hostUID), GID: int(hostGID)}
	dest := mountPoint
	if destination != "" && filepath.IsAbs(destination) {
		dest = filepath.Join(dest, destination)
	} else {
		if err = idtools.MkdirAllAndChownNew(filepath.Join(dest, b.WorkDir()), 0755, hostOwner); err != nil {
			return err
		}
		dest = filepath.Join(dest, b.WorkDir(), destination)
	}
	// If the destination was explicitly marked as a directory by ending it
	// with a '/', create it so that we can be sure that it's a directory,
	// and any files we're copying will be placed in the directory.
	if len(destination) > 0 && destination[len(destination)-1] == os.PathSeparator {
		if err = idtools.MkdirAllAndChownNew(dest, 0755, hostOwner); err != nil {
			return err
		}
	}
	// Make sure the destination's parent directory is usable.
	if destpfi, err2 := os.Stat(filepath.Dir(dest)); err2 == nil && !destpfi.IsDir() {
		return errors.Errorf("%q already exists, but is not a subdirectory)", filepath.Dir(dest))
	}
	// Now look at the destination itself.
	destfi, err := os.Stat(dest)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't determine what %q is", dest)
		}
		destfi = nil
	}
	if len(source) > 1 && (destfi == nil || !destfi.IsDir()) {
		return errors.Errorf("destination %q is not a directory", dest)
	}
	copyFileWithTar := b.copyFileWithTar(&containerOwner, options.Hasher)
	copyWithTar := b.copyWithTar(&containerOwner, options.Hasher)
	untarPath := b.untarPath(nil, options.Hasher)
	for _, src := range source {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			// We assume that source is a file, and we're copying
			// it to the destination.  If the destination is
			// already a directory, create a file inside of it.
			// Otherwise, the destination is the file to which
			// we'll save the contents.
			url, err := url.Parse(src)
			if err != nil {
				return errors.Wrapf(err, "error parsing URL %q", src)
			}
			d := dest
			if destfi != nil && destfi.IsDir() {
				d = filepath.Join(dest, path.Base(url.Path))
			}
			if err := addURL(d, src, hostOwner, options.Hasher); err != nil {
				return err
			}
			continue
		}

		glob, err := filepath.Glob(src)
		if err != nil {
			return errors.Wrapf(err, "invalid glob %q", src)
		}
		if len(glob) == 0 {
			return errors.Wrapf(syscall.ENOENT, "no files found matching %q", src)
		}
		for _, gsrc := range glob {
			srcfi, err := os.Stat(gsrc)
			if err != nil {
				return errors.Wrapf(err, "error reading %q", gsrc)
			}
			if srcfi.IsDir() {
				// The source is a directory, so copy the contents of
				// the source directory into the target directory.  Try
				// to create it first, so that if there's a problem,
				// we'll discover why that won't work.
				if err = idtools.MkdirAllAndChownNew(dest, 0755, hostOwner); err != nil {
					return err
				}
				logrus.Debugf("copying %q to %q", gsrc+string(os.PathSeparator)+"*", dest+string(os.PathSeparator)+"*")
				if err := copyWithTar(gsrc, dest); err != nil {
					return errors.Wrapf(err, "error copying %q to %q", gsrc, dest)
				}
				continue
			}
			if !extract || !archive.IsArchivePath(gsrc) {
				// This source is a file, and either it's not an
				// archive, or we don't care whether or not it's an
				// archive.
				d := dest
				if destfi != nil && destfi.IsDir() {
					d = filepath.Join(dest, filepath.Base(gsrc))
				}
				// Copy the file, preserving attributes.
				logrus.Debugf("copying %q to %q", gsrc, d)
				if err := copyFileWithTar(gsrc, d); err != nil {
					return errors.Wrapf(err, "error copying %q to %q", gsrc, d)
				}
				continue
			}
			// We're extracting an archive into the destination directory.
			logrus.Debugf("extracting contents of %q into %q", gsrc, dest)
			if err := untarPath(gsrc, dest); err != nil {
				return errors.Wrapf(err, "error extracting %q into %q", gsrc, dest)
			}
		}
	}
	return nil
}

// user returns the user (and group) information which the destination should belong to.
func (b *Builder) user(mountPoint string, userspec string) (specs.User, error) {
	if userspec == "" {
		userspec = b.User()
	}

	uid, gid, err := chrootuser.GetUser(mountPoint, userspec)
	u := specs.User{
		UID:      uid,
		GID:      gid,
		Username: userspec,
	}
	if !strings.Contains(userspec, ":") {
		groups, err2 := chrootuser.GetAdditionalGroupsForUser(mountPoint, uint64(u.UID))
		if err2 != nil {
			if errors.Cause(err2) != chrootuser.ErrNoSuchUser && err == nil {
				err = err2
			}
		} else {
			u.AdditionalGids = groups
		}

	}
	return u, err
}
