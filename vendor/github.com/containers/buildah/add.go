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
	"github.com/containers/storage/pkg/fileutils"
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
	// Excludes is the contents of the .dockerignore file
	Excludes []string
	// ContextDir is the base directory for Excludes for content being copied
	ContextDir string
	// ID mapping options to use when contents to be copied are part of
	// another container, and need ownerships to be mapped from the host to
	// that container's values before copying them into the container.
	IDMappingOptions *IDMappingOptions
	// DryRun indicates that the content should be digested, but not actually
	// copied into the container.
	DryRun bool
}

// addURL copies the contents of the source URL to the destination.  This is
// its own function so that deferred closes happen after we're done pulling
// down each item of potentially many.
func (b *Builder) addURL(destination, srcurl string, owner idtools.IDPair, hasher io.Writer, dryRun bool) error {
	resp, err := http.Get(srcurl)
	if err != nil {
		return errors.Wrapf(err, "error getting %q", srcurl)
	}
	defer resp.Body.Close()

	thisHasher := hasher
	if thisHasher != nil && b.ContentDigester.Hash() != nil {
		thisHasher = io.MultiWriter(thisHasher, b.ContentDigester.Hash())
	}
	if thisHasher == nil {
		thisHasher = b.ContentDigester.Hash()
	}
	thisWriter := thisHasher

	if !dryRun {
		logrus.Debugf("saving %q to %q", srcurl, destination)
		f, err := os.Create(destination)
		if err != nil {
			return errors.Wrapf(err, "error creating %q", destination)
		}
		defer f.Close()
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
		defer func() {
			if err2 := f.Chmod(0600); err2 != nil {
				logrus.Debugf("error setting permissions on %q: %v", destination, err2)
			}
		}()
		thisWriter = io.MultiWriter(f, thisWriter)
	}

	n, err := io.Copy(thisWriter, resp.Body)
	if err != nil {
		return errors.Wrapf(err, "error reading contents for %q from %q", destination, srcurl)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return errors.Errorf("error reading contents for %q from %q: wrong length (%d != %d)", destination, srcurl, n, resp.ContentLength)
	}
	return nil
}

// Add copies the contents of the specified sources into the container's root
// filesystem, optionally extracting contents of local files that look like
// non-empty archives.
func (b *Builder) Add(destination string, extract bool, options AddAndCopyOptions, source ...string) error {
	excludes, err := dockerIgnoreMatcher(options.Excludes, options.ContextDir)
	if err != nil {
		return err
	}
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
	user, _, err := b.user(mountPoint, options.Chown)
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
	if !options.DryRun {
		// Resolve the destination if it was specified as a relative path.
		if destination != "" && filepath.IsAbs(destination) {
			dir := filepath.Dir(destination)
			if dir != "." && dir != "/" {
				if err = idtools.MkdirAllAndChownNew(filepath.Join(dest, dir), 0755, hostOwner); err != nil {
					return errors.Wrapf(err, "error creating directory %q", filepath.Join(dest, dir))
				}
			}
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
	copyFileWithTar := b.copyFileWithTar(options.IDMappingOptions, &containerOwner, options.Hasher, options.DryRun)
	copyWithTar := b.copyWithTar(options.IDMappingOptions, &containerOwner, options.Hasher, options.DryRun)
	untarPath := b.untarPath(nil, options.Hasher, options.DryRun)
	err = b.addHelper(excludes, extract, dest, destfi, hostOwner, options, copyFileWithTar, copyWithTar, untarPath, source...)
	if err != nil {
		return err
	}
	return nil
}

// user returns the user (and group) information which the destination should belong to.
func (b *Builder) user(mountPoint string, userspec string) (specs.User, string, error) {
	if userspec == "" {
		userspec = b.User()
	}

	uid, gid, homeDir, err := chrootuser.GetUser(mountPoint, userspec)
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
	return u, homeDir, err
}

// dockerIgnoreMatcher returns a matcher based on the contents of the .dockerignore file under contextDir
func dockerIgnoreMatcher(lines []string, contextDir string) (*fileutils.PatternMatcher, error) {
	// if there's no context dir, there's no .dockerignore file to consult
	if contextDir == "" {
		return nil, nil
	}
	// If there's no .dockerignore file, then we don't have to add a
	// pattern to tell copy logic to ignore it later.
	var patterns []string
	if _, err := os.Stat(filepath.Join(contextDir, ".dockerignore")); err == nil || !os.IsNotExist(err) {
		patterns = []string{".dockerignore"}
	}
	for _, ignoreSpec := range lines {
		ignoreSpec = strings.TrimSpace(ignoreSpec)
		// ignore comments passed back from .dockerignore
		if ignoreSpec == "" || ignoreSpec[0] == '#' {
			continue
		}
		// if the spec starts with '!' it means the pattern
		// should be included. make a note so that we can move
		// it to the front of the updated pattern, and insert
		// the context dir's path in between
		includeFlag := ""
		if strings.HasPrefix(ignoreSpec, "!") {
			includeFlag = "!"
			ignoreSpec = ignoreSpec[1:]
		}
		if ignoreSpec == "" {
			continue
		}
		patterns = append(patterns, includeFlag+filepath.Join(contextDir, ignoreSpec))
	}
	// if there are no patterns, save time by not constructing the object
	if len(patterns) == 0 {
		return nil, nil
	}
	// return a matcher object
	matcher, err := fileutils.NewPatternMatcher(patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating file matcher using patterns %v", patterns)
	}
	return matcher, nil
}

func (b *Builder) addHelper(excludes *fileutils.PatternMatcher, extract bool, dest string, destfi os.FileInfo, hostOwner idtools.IDPair, options AddAndCopyOptions, copyFileWithTar, copyWithTar, untarPath func(src, dest string) error, source ...string) error {
	for n, src := range source {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			b.ContentDigester.Start("")
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
			if err = b.addURL(d, src, hostOwner, options.Hasher, options.DryRun); err != nil {
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
			esrc, err := filepath.EvalSymlinks(gsrc)
			if err != nil {
				return errors.Wrapf(err, "error evaluating symlinks %q", gsrc)
			}
			srcfi, err := os.Stat(esrc)
			if err != nil {
				return errors.Wrapf(err, "error reading %q", esrc)
			}
			if srcfi.IsDir() {
				b.ContentDigester.Start("dir")
				// The source is a directory, so copy the contents of
				// the source directory into the target directory.  Try
				// to create it first, so that if there's a problem,
				// we'll discover why that won't work.
				if !options.DryRun {
					if err = idtools.MkdirAllAndChownNew(dest, 0755, hostOwner); err != nil {
						return errors.Wrapf(err, "error creating directory %q", dest)
					}
				}
				logrus.Debugf("copying[%d] %q to %q", n, esrc+string(os.PathSeparator)+"*", dest+string(os.PathSeparator)+"*")

				// Copy the whole directory because we do not exclude anything
				if excludes == nil {
					if err = copyWithTar(esrc, dest); err != nil {
						return errors.Wrapf(err, "error copying %q to %q", esrc, dest)
					}
					continue
				}
				err := filepath.Walk(esrc, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					res, err := excludes.MatchesResult(path)
					if err != nil {
						return errors.Wrapf(err, "error checking if %s is an excluded path", path)
					}
					// The latest match result has the highest priority,
					// which means that we only skip the filepath if
					// the last result matched.
					if res.IsMatched() {
						return nil
					}

					// combine the source's basename with the dest directory
					fpath, err := filepath.Rel(esrc, path)
					if err != nil {
						return errors.Wrapf(err, "error converting %s to a path relative to %s", path, esrc)
					}
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

			// This source is a file
			// Check if the path matches the .dockerignore
			if excludes != nil {
				res, err := excludes.MatchesResult(esrc)
				if err != nil {
					return errors.Wrapf(err, "error checking if %s is an excluded path", esrc)
				}
				// Skip the file if the pattern matches
				if res.IsMatched() {
					continue
				}
			}

			b.ContentDigester.Start("file")

			if !extract || !archive.IsArchivePath(esrc) {
				// This source is a file, and either it's not an
				// archive, or we don't care whether or not it's an
				// archive.
				d := dest
				if destfi != nil && destfi.IsDir() {
					d = filepath.Join(dest, filepath.Base(gsrc))
				}
				// Copy the file, preserving attributes.
				logrus.Debugf("copying[%d] %q to %q", n, esrc, d)
				if err = copyFileWithTar(esrc, d); err != nil {
					return errors.Wrapf(err, "error copying %q to %q", esrc, d)
				}
				continue
			}

			// We're extracting an archive into the destination directory.
			logrus.Debugf("extracting contents[%d] of %q into %q", n, esrc, dest)
			if err = untarPath(esrc, dest); err != nil {
				return errors.Wrapf(err, "error extracting %q into %q", esrc, dest)
			}
		}
	}
	return nil
}
