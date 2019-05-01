package imagebuildah

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	symlinkChrootedCommand = "chrootsymlinks-resolve"
	symlinkModifiedTime    = "modtimesymlinks-resolve"
	maxSymlinksResolved    = 40
)

func init() {
	reexec.Register(symlinkChrootedCommand, resolveChrootedSymlinks)
	reexec.Register(symlinkModifiedTime, resolveSymlinkTimeModified)
}

// resolveSymlink uses a child subprocess to resolve any symlinks in filename
// in the context of rootdir.
func resolveSymlink(rootdir, filename string) (string, error) {
	// The child process expects a chroot and one path that
	// will be consulted relative to the chroot directory and evaluated
	// for any symbolic links present.
	cmd := reexec.Command(symlinkChrootedCommand, rootdir, filename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, string(output))
	}

	// Hand back the resolved symlink, will be filename if a symlink is not found
	return string(output), nil
}

// main() for resolveSymlink()'s subprocess.
func resolveChrootedSymlinks() {
	status := 0
	flag.Parse()
	if len(flag.Args()) < 2 {
		fmt.Fprintf(os.Stderr, "%s needs two arguments\n", symlinkChrootedCommand)
		os.Exit(1)
	}
	// Our first parameter is the directory to chroot into.
	if err := unix.Chdir(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chdir(): %v\n", err)
		os.Exit(1)
	}
	if err := unix.Chroot(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chroot(): %v\n", err)
		os.Exit(1)
	}

	// Our second parameter is the path name to evaluate for symbolic links
	symLink, err := getSymbolicLink(flag.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting symbolic links: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.WriteString(symLink); err != nil {
		fmt.Fprintf(os.Stderr, "error writing string to stdout: %v\n", err)
		os.Exit(1)
	}
	os.Exit(status)
}

// main() for grandparent subprocess.  Its main job is to shuttle stdio back
// and forth, managing a pseudo-terminal if we want one, for our child, the
// parent subprocess.
func resolveSymlinkTimeModified() {
	status := 0
	flag.Parse()
	if len(flag.Args()) < 1 {
		os.Exit(1)
	}
	// Our first parameter is the directory to chroot into.
	if err := unix.Chdir(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chdir(): %v\n", err)
		os.Exit(1)
	}
	if err := unix.Chroot(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chroot(): %v\n", err)
		os.Exit(1)
	}

	// Our second parameter is the path name to evaluate for symbolic links.
	// Our third parameter is the time the cached intermediate image was created.
	// We check whether the modified time of the filepath we provide is after the time the cached image was created.
	timeIsGreater, err := modTimeIsGreater(flag.Arg(0), flag.Arg(1), flag.Arg(2))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking if modified time of resolved symbolic link is greater: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.WriteString(fmt.Sprintf("%v", timeIsGreater)); err != nil {
		fmt.Fprintf(os.Stderr, "error writing string to stdout: %v\n", err)
		os.Exit(1)
	}
	os.Exit(status)
}

// resolveModifiedTime (in the grandparent process) checks filename for any symlinks,
// resolves it and compares the modified time of the file with historyTime, which is
// the creation time of the cached image. It returns true if filename was modified after
// historyTime, otherwise returns false.
func resolveModifiedTime(rootdir, filename, historyTime string) (bool, error) {
	// The child process expects a chroot and one path that
	// will be consulted relative to the chroot directory and evaluated
	// for any symbolic links present.
	cmd := reexec.Command(symlinkModifiedTime, rootdir, filename, historyTime)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, errors.Wrapf(err, string(output))
	}
	// Hand back true/false depending on in the file was modified after the caches image was created.
	return string(output) == "true", nil
}

// modTimeIsGreater goes through the files added/copied in using the Dockerfile and
// checks the time stamp (follows symlinks) with the time stamp of when the cached
// image was created. IT compares the two and returns true if the file was modified
// after the cached image was created, otherwise it returns false.
func modTimeIsGreater(rootdir, path string, historyTime string) (bool, error) {
	var timeIsGreater bool

	// Convert historyTime from string to time.Time for comparison
	histTime, err := time.Parse(time.RFC3339Nano, historyTime)
	if err != nil {
		return false, errors.Wrapf(err, "error converting string to time.Time %q", historyTime)
	}

	// Since we are chroot in rootdir, we want a relative path, i.e (path - rootdir)
	relPath, err := filepath.Rel(rootdir, path)
	if err != nil {
		return false, errors.Wrapf(err, "error making path %q relative to %q", path, rootdir)
	}

	// Walk the file tree and check the time stamps.
	err = filepath.Walk(relPath, func(path string, info os.FileInfo, err error) error {
		// If using cached images, it is possible for files that are being copied to come from
		// previous build stages. But if using cached images, then the copied file won't exist
		// since a container won't have been created for the previous build stage and info will be nil.
		// In that case just return nil and continue on with using the cached image for the whole build process.
		if info == nil {
			return nil
		}
		modTime := info.ModTime()
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			// Evaluate any symlink that occurs to get updated modified information
			resolvedPath, err := filepath.EvalSymlinks(path)
			if err != nil && os.IsNotExist(err) {
				return errors.Wrapf(errDanglingSymlink, "%q", path)
			}
			if err != nil {
				return errors.Wrapf(err, "error evaluating symlink %q", path)
			}
			fileInfo, err := os.Stat(resolvedPath)
			if err != nil {
				return errors.Wrapf(err, "error getting file info %q", resolvedPath)
			}
			modTime = fileInfo.ModTime()
		}
		if modTime.After(histTime) {
			timeIsGreater = true
			return nil
		}
		return nil
	})

	if err != nil {
		// if error is due to dangling symlink, ignore error and return nil
		if errors.Cause(err) == errDanglingSymlink {
			return false, nil
		}
		return false, errors.Wrapf(err, "error walking file tree %q", path)
	}
	return timeIsGreater, err
}

// getSymbolic link goes through each part of the path and continues resolving symlinks as they appear.
// Returns what the whole target path for what "path" resolves to.
func getSymbolicLink(path string) (string, error) {
	var (
		symPath          string
		symLinksResolved int
	)
	// Splitting path as we need to resolve each part of the path at a time
	splitPath := strings.Split(path, "/")
	if splitPath[0] == "" {
		splitPath = splitPath[1:]
		symPath = "/"
	}
	for _, p := range splitPath {
		// If we have resolved 40 symlinks, that means something is terribly wrong
		// will return an error and exit
		if symLinksResolved >= maxSymlinksResolved {
			return "", errors.Errorf("have resolved %q symlinks, something is terribly wrong!", maxSymlinksResolved)
		}
		symPath = filepath.Join(symPath, p)
		isSymlink, resolvedPath, err := hasSymlink(symPath)
		if err != nil {
			return "", errors.Wrapf(err, "error checking symlink for %q", symPath)
		}
		// if isSymlink is true, check if resolvedPath is potentially another symlink
		// keep doing this till resolvedPath is not a symlink and isSymlink is false
		for isSymlink == true {
			// Need to keep track of number of symlinks resolved
			// Will also return an error if the symlink points to itself as that will exceed maxSymlinksResolved
			if symLinksResolved >= maxSymlinksResolved {
				return "", errors.Errorf("have resolved %q symlinks, something is terribly wrong!", maxSymlinksResolved)
			}
			isSymlink, resolvedPath, err = hasSymlink(resolvedPath)
			if err != nil {
				return "", errors.Wrapf(err, "error checking symlink for %q", resolvedPath)
			}
			symLinksResolved++
		}
		// Assign resolvedPath to symPath. The next part of the loop will append the next part of the original path
		// and continue resolving
		symPath = resolvedPath
		symLinksResolved++
	}
	return symPath, nil
}

// hasSymlink returns true and the target if path is symlink
// otherwise it returns false and path
func hasSymlink(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(path, 0755); err != nil {
				return false, "", errors.Wrapf(err, "error ensuring volume path %q exists", path)
			}
			info, err = os.Lstat(path)
			if err != nil {
				return false, "", errors.Wrapf(err, "error running lstat on %q", path)
			}
		} else {
			return false, path, errors.Wrapf(err, "error get stat of path %q", path)
		}
	}

	// Return false and path as path if not a symlink
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return false, path, nil
	}

	// Read the symlink to get what it points to
	targetDir, err := os.Readlink(path)
	if err != nil {
		return false, "", errors.Wrapf(err, "error reading link %q", path)
	}
	// if the symlink points to a relative path, prepend the path till now to the resolved path
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(filepath.Dir(path), targetDir)
	}
	// run filepath.Clean to remove the ".." from relative paths
	return true, filepath.Clean(targetDir), nil
}
