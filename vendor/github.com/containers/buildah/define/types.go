package define

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package. Also used by .packit.sh for Packit builds.
	Version = "1.31.0"

	// DefaultRuntime if containers.conf fails.
	DefaultRuntime = "runc"

	// OCIv1ImageManifest is the MIME type of an OCIv1 image manifest,
	// suitable for specifying as a value of the PreferredManifestType
	// member of a CommitOptions structure.  It is also the default.
	OCIv1ImageManifest = v1.MediaTypeImageManifest
	// Dockerv2ImageManifest is the MIME type of a Docker v2s2 image
	// manifest, suitable for specifying as a value of the
	// PreferredManifestType member of a CommitOptions structure.
	Dockerv2ImageManifest = manifest.DockerV2Schema2MediaType

	// OCI used to define the "oci" image format
	OCI = "oci"
	// DOCKER used to define the "docker" image format
	DOCKER = "docker"
)

var (
	// DefaultCapabilities is the list of capabilities which we grant by
	// default to containers which are running under UID 0.
	DefaultCapabilities = []string{
		"CAP_AUDIT_WRITE",
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_MKNOD",
		"CAP_NET_BIND_SERVICE",
		"CAP_SETFCAP",
		"CAP_SETGID",
		"CAP_SETPCAP",
		"CAP_SETUID",
		"CAP_SYS_CHROOT",
	}
	// DefaultNetworkSysctl is the list of Kernel parameters which we
	// grant by default to containers which are running under UID 0.
	DefaultNetworkSysctl = map[string]string{
		"net.ipv4.ping_group_range": "0 0",
	}

	Gzip         = archive.Gzip
	Bzip2        = archive.Bzip2
	Xz           = archive.Xz
	Zstd         = archive.Zstd
	Uncompressed = archive.Uncompressed
)

// IDMappingOptions controls how we set up UID/GID mapping when we set up a
// user namespace.
type IDMappingOptions struct {
	HostUIDMapping bool
	HostGIDMapping bool
	UIDMap         []specs.LinuxIDMapping
	GIDMap         []specs.LinuxIDMapping
	AutoUserNs     bool
	AutoUserNsOpts types.AutoUserNsOptions
}

// Secret is a secret source that can be used in a RUN
type Secret struct {
	ID         string
	Source     string
	SourceType string
}

// BuildOutputOptions contains the the outcome of parsing the value of a build --output flag
type BuildOutputOption struct {
	Path     string // Only valid if !IsStdout
	IsDir    bool
	IsStdout bool
}

// TempDirForURL checks if the passed-in string looks like a URL or -.  If it is,
// TempDirForURL creates a temporary directory, arranges for its contents to be
// the contents of that URL, and returns the temporary directory's path, along
// with the name of a subdirectory which should be used as the build context
// (which may be empty or ".").  Removal of the temporary directory is the
// responsibility of the caller.  If the string doesn't look like a URL,
// TempDirForURL returns empty strings and a nil error code.
func TempDirForURL(dir, prefix, url string) (name string, subdir string, err error) {
	if !strings.HasPrefix(url, "http://") &&
		!strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "git://") &&
		!strings.HasPrefix(url, "github.com/") &&
		url != "-" {
		return "", "", nil
	}
	name, err = os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", "", fmt.Errorf("creating temporary directory for %q: %w", url, err)
	}
	urlParsed, err := urlpkg.Parse(url)
	if err != nil {
		return "", "", fmt.Errorf("parsing url %q: %w", url, err)
	}
	if strings.HasPrefix(url, "git://") || strings.HasSuffix(urlParsed.Path, ".git") {
		combinedOutput, gitSubDir, err := cloneToDirectory(url, name)
		if err != nil {
			if err2 := os.RemoveAll(name); err2 != nil {
				logrus.Debugf("error removing temporary directory %q: %v", name, err2)
			}
			return "", "", fmt.Errorf("cloning %q to %q:\n%s: %w", url, name, string(combinedOutput), err)
		}
		return name, gitSubDir, nil
	}
	if strings.HasPrefix(url, "github.com/") {
		ghurl := url
		url = fmt.Sprintf("https://%s/archive/master.tar.gz", ghurl)
		logrus.Debugf("resolving url %q to %q", ghurl, url)
		subdir = path.Base(ghurl) + "-master"
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		err = downloadToDirectory(url, name)
		if err != nil {
			if err2 := os.RemoveAll(name); err2 != nil {
				logrus.Debugf("error removing temporary directory %q: %v", name, err2)
			}
			return "", subdir, err
		}
		return name, subdir, nil
	}
	if url == "-" {
		err = stdinToDirectory(name)
		if err != nil {
			if err2 := os.RemoveAll(name); err2 != nil {
				logrus.Debugf("error removing temporary directory %q: %v", name, err2)
			}
			return "", subdir, err
		}
		logrus.Debugf("Build context is at %q", name)
		return name, subdir, nil
	}
	logrus.Debugf("don't know how to retrieve %q", url)
	if err2 := os.Remove(name); err2 != nil {
		logrus.Debugf("error removing temporary directory %q: %v", name, err2)
	}
	return "", "", errors.New("unreachable code reached")
}

// parseGitBuildContext parses git build context to `repo`, `sub-dir`
// `branch/commit`, accepts GitBuildContext in the format of
// `repourl.git[#[branch-or-commit]:subdir]`.
func parseGitBuildContext(url string) (string, string, string) {
	gitSubdir := ""
	gitBranch := ""
	gitBranchPart := strings.Split(url, "#")
	if len(gitBranchPart) > 1 {
		// check if string contains path to a subdir
		gitSubDirPart := strings.Split(gitBranchPart[1], ":")
		if len(gitSubDirPart) > 1 {
			gitSubdir = gitSubDirPart[1]
		}
		gitBranch = gitSubDirPart[0]
	}
	return gitBranchPart[0], gitSubdir, gitBranch
}

func cloneToDirectory(url, dir string) ([]byte, string, error) {
	var cmd *exec.Cmd
	gitRepo, gitSubdir, gitBranch := parseGitBuildContext(url)
	// init repo
	cmd = exec.Command("git", "init", dir)
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git init`: %w", err)
	}
	// add origin
	cmd = exec.Command("git", "remote", "add", "origin", gitRepo)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git remote add`: %w", err)
	}
	// fetch required branch or commit and perform checkout
	// Always default to `HEAD` if nothing specified
	fetch := "HEAD"
	if gitBranch != "" {
		fetch = gitBranch
	}
	logrus.Debugf("fetching repo %q and branch (or commit ID) %q to %q", gitRepo, fetch, dir)
	cmd = exec.Command("git", "fetch", "--depth=1", "origin", "--", fetch)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git fetch`: %w", err)
	}
	if fetch == "HEAD" {
		// We fetched default branch therefore
		// we don't have any valid `branch` or
		// `commit` name hence checkout detached
		// `FETCH_HEAD`
		fetch = "FETCH_HEAD"
	}
	cmd = exec.Command("git", "checkout", fetch)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git checkout`: %w", err)
	}
	return combinedOutput, gitSubdir, nil
}

func downloadToDirectory(url, dir string) error {
	logrus.Debugf("extracting %q to %q", url, dir)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("invalid response status %d", resp.StatusCode)
	}
	if resp.ContentLength == 0 {
		return fmt.Errorf("no contents in %q", url)
	}
	if err := chrootarchive.Untar(resp.Body, dir, nil); err != nil {
		resp1, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp1.Body.Close()
		body, err := io.ReadAll(resp1.Body)
		if err != nil {
			return err
		}
		dockerfile := filepath.Join(dir, "Dockerfile")
		// Assume this is a Dockerfile
		if err := ioutils.AtomicWriteFile(dockerfile, body, 0600); err != nil {
			return fmt.Errorf("failed to write %q to %q: %w", url, dockerfile, err)
		}
	}
	return nil
}

func stdinToDirectory(dir string) error {
	logrus.Debugf("extracting stdin to %q", dir)
	r := bufio.NewReader(os.Stdin)
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}
	reader := bytes.NewReader(b)
	if err := chrootarchive.Untar(reader, dir, nil); err != nil {
		dockerfile := filepath.Join(dir, "Dockerfile")
		// Assume this is a Dockerfile
		if err := ioutils.AtomicWriteFile(dockerfile, b, 0600); err != nil {
			return fmt.Errorf("failed to write bytes to %q: %w", dockerfile, err)
		}
	}
	return nil
}
