package imagebuildah

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	symlinkChrootedCommand = "chrootsymlinks-resolve"
	maxSymlinksResolved    = 40
)

func init() {
	reexec.Register(symlinkChrootedCommand, resolveChrootedSymlinks)
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
			return "", err
		}
		// if isSymlink is true, check if resolvedPath is potentially another symlink
		// keep doing this till resolvedPath is not a symlink and isSymlink is false
		for isSymlink {
			// Need to keep track of number of symlinks resolved
			// Will also return an error if the symlink points to itself as that will exceed maxSymlinksResolved
			if symLinksResolved >= maxSymlinksResolved {
				return "", errors.Errorf("have resolved %q symlinks, something is terribly wrong!", maxSymlinksResolved)
			}
			isSymlink, resolvedPath, err = hasSymlink(resolvedPath)
			if err != nil {
				return "", err
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
				return false, "", err
			}
			info, err = os.Lstat(path)
			if err != nil {
				return false, "", err
			}
		} else {
			return false, path, err
		}
	}

	// Return false and path as path if not a symlink
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return false, path, nil
	}

	// Read the symlink to get what it points to
	targetDir, err := os.Readlink(path)
	if err != nil {
		return false, "", err
	}
	// if the symlink points to a relative path, prepend the path till now to the resolved path
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(filepath.Dir(path), targetDir)
	}
	// run filepath.Clean to remove the ".." from relative paths
	return true, filepath.Clean(targetDir), nil
}
