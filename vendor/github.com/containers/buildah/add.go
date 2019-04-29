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

	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
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
	// Exludes contents in the .dockerignore file
	Excludes []string
	// current directory on host
	ContextDir string
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
		return errors.Wrapf(err, "error setting owner of %q to %d:%d", destination, owner.UID, owner.GID)
	}
	if last := resp.Header.Get("Last-Modified"); last != "" {
		if mtime, err2 := time.Parse(time.RFC1123, last); err2 != nil {
			logrus.Debugf("error parsing Last-Modified time %q: %v", last, err2)
		} else {
			defer func() {
				if err3 := os.Chtimes(destination, time.Now(), mtime); err3 != nil {
					logrus.Debugf("error setting mtime on %q to Last-Modified time %q: %v", destination, last, err3)
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
		return errors.Wrapf(err, "error reading contents for %q from %q", destination, srcurl)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return errors.Errorf("error reading contents for %q from %q: wrong length (%d != %d)", destination, srcurl, n, resp.ContentLength)
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
	excludes := DockerIgnoreHelper(options.Excludes, options.ContextDir)
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
			return errors.Wrapf(err, "error creating directory %q", filepath.Join(dest, b.WorkDir()))
		}
		dest = filepath.Join(dest, b.WorkDir(), destination)
	}
	// If the destination was explicitly marked as a directory by ending it
	// with a '/', create it so that we can be sure that it's a directory,
	// and any files we're copying will be placed in the directory.
	if len(destination) > 0 && destination[len(destination)-1] == os.PathSeparator {
		if err = idtools.MkdirAllAndChownNew(dest, 0755, hostOwner); err != nil {
			return errors.Wrapf(err, "error creating directory %q", dest)
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
	err = addHelper(excludes, extract, dest, destfi, hostOwner, options, copyFileWithTar, copyWithTar, untarPath, source...)
	if err != nil {
		return err
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

// DockerIgnore struct keep info from .dockerignore
type DockerIgnore struct {
	ExcludePath string
	IsExcluded  bool
}

// DockerIgnoreHelper returns the lines from .dockerignore file without the comments
// and reverses the order
func DockerIgnoreHelper(lines []string, contextDir string) []DockerIgnore {
	var excludes []DockerIgnore
	// the last match of a file in the .dockerignmatches determines whether it is included or excluded
	// reverse the order
	for i := len(lines) - 1; i >= 0; i-- {
		exclude := lines[i]
		// ignore the comment in .dockerignore
		if strings.HasPrefix(exclude, "#") || len(exclude) == 0 {
			continue
		}
		excludeFlag := true
		if strings.HasPrefix(exclude, "!") {
			exclude = strings.TrimPrefix(exclude, "!")
			excludeFlag = false
		}
		excludes = append(excludes, DockerIgnore{ExcludePath: filepath.Join(contextDir, exclude), IsExcluded: excludeFlag})
	}
	if len(excludes) != 0 {
		excludes = append(excludes, DockerIgnore{ExcludePath: filepath.Join(contextDir, ".dockerignore"), IsExcluded: true})
	}
	return excludes
}

func addHelper(excludes []DockerIgnore, extract bool, dest string, destfi os.FileInfo, hostOwner idtools.IDPair, options AddAndCopyOptions, copyFileWithTar, copyWithTar, untarPath func(src, dest string) error, source ...string) error {
	dirsInDockerignore, err := getDirsInDockerignore(options.ContextDir, excludes)
	if err != nil {
		return errors.Wrapf(err, "error checking directories in .dockerignore")
	}
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
			if err = addURL(d, src, hostOwner, options.Hasher); err != nil {
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
	outer:
		for _, gsrc := range glob {
			esrc, err := filepath.EvalSymlinks(gsrc)
			if err != nil {
				return errors.Wrapf(err, "error evaluating symlinks %q", gsrc)
			}
			srcfi, err := os.Stat(esrc)
			if err != nil {
				return errors.Wrapf(err, "error reading %q", esrc)
			}
			if srcfi.IsDir() {
				// The source is a directory, so copy the contents of
				// the source directory into the target directory.  Try
				// to create it first, so that if there's a problem,
				// we'll discover why that won't work.
				if err = idtools.MkdirAllAndChownNew(dest, 0755, hostOwner); err != nil {
					return errors.Wrapf(err, "error creating directory %q", dest)
				}
				logrus.Debugf("copying %q to %q", esrc+string(os.PathSeparator)+"*", dest+string(os.PathSeparator)+"*")
				if len(excludes) == 0 {
					if err = copyWithTar(esrc, dest); err != nil {
						return errors.Wrapf(err, "error copying %q to %q", esrc, dest)
					}
					continue
				}
				err := filepath.Walk(esrc, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}
					for _, exclude := range excludes {
						match, err := filepath.Match(filepath.Clean(exclude.ExcludePath), filepath.Clean(path))
						if err != nil {
							return err
						}
						prefix, exist := dirsInDockerignore[exclude.ExcludePath]
						hasPrefix := false
						if exist {
							hasPrefix = filepath.HasPrefix(path, prefix)
						}
						if !(match || hasPrefix) {
							continue
						}
						if (hasPrefix && exclude.IsExcluded) || (match && exclude.IsExcluded) {
							return nil
						}
						break
					}
					// combine the filename with the dest directory
					fpath := strings.TrimPrefix(path, esrc)
					if err = copyFileWithTar(path, filepath.Join(dest, fpath)); err != nil {
						return errors.Wrapf(err, "error copying %q to %q", path, dest)
					}
					return nil
				})
				if err != nil {
					return err
				}
				continue
			}

			for _, exclude := range excludes {
				match, err := filepath.Match(filepath.Clean(exclude.ExcludePath), esrc)
				if err != nil {
					return err
				}
				if !match {
					continue
				}
				if exclude.IsExcluded {
					continue outer
				}
				break
			}

			if !extract || !archive.IsArchivePath(esrc) {
				// This source is a file, and either it's not an
				// archive, or we don't care whether or not it's an
				// archive.
				d := dest
				if destfi != nil && destfi.IsDir() {
					d = filepath.Join(dest, filepath.Base(gsrc))
				}
				// Copy the file, preserving attributes.
				logrus.Debugf("copying %q to %q", esrc, d)
				if err = copyFileWithTar(esrc, d); err != nil {
					return errors.Wrapf(err, "error copying %q to %q", esrc, d)
				}
				continue
			}
			// We're extracting an archive into the destination directory.
			logrus.Debugf("extracting contents of %q into %q", esrc, dest)
			if err = untarPath(esrc, dest); err != nil {
				return errors.Wrapf(err, "error extracting %q into %q", esrc, dest)
			}
		}
	}
	return nil
}

func getDirsInDockerignore(srcAbsPath string, excludes []DockerIgnore) (map[string]string, error) {
	visitedDir := make(map[string]string)
	if len(excludes) == 0 {
		return visitedDir, nil
	}
	err := filepath.Walk(srcAbsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, exclude := range excludes {
				match, err := filepath.Match(filepath.Clean(exclude.ExcludePath), filepath.Clean(path))
				if err != nil {
					return err
				}
				if !match {
					continue
				}
				if _, exist := visitedDir[exclude.ExcludePath]; exist {
					continue
				}
				visitedDir[exclude.ExcludePath] = path
			}
		}
		return nil
	})
	if err != nil {
		return visitedDir, err
	}
	return visitedDir, nil
}
