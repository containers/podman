package define

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/ioutils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package.  Bump version in contrib/rpm/buildah.spec
	// too.
	Version = "1.21.0"

	// DefaultRuntime if containers.conf fails.
	DefaultRuntime = "runc"

	DefaultCNIPluginPath = "/usr/libexec/cni:/opt/cni/bin"
	// DefaultCNIConfigDir is the default location of CNI configuration files.
	DefaultCNIConfigDir = "/etc/cni/net.d"

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
	name, err = ioutil.TempDir(dir, prefix)
	if err != nil {
		return "", "", errors.Wrapf(err, "error creating temporary directory for %q", url)
	}
	if strings.HasPrefix(url, "git://") || strings.HasSuffix(url, ".git") {
		err = cloneToDirectory(url, name)
		if err != nil {
			if err2 := os.RemoveAll(name); err2 != nil {
				logrus.Debugf("error removing temporary directory %q: %v", name, err2)
			}
			return "", "", err
		}
		return name, "", nil
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
	return "", "", errors.Errorf("unreachable code reached")
}

func cloneToDirectory(url, dir string) error {
	if !strings.HasPrefix(url, "git://") && !strings.HasSuffix(url, ".git") {
		url = "git://" + url
	}
	gitBranch := strings.Split(url, "#")
	var cmd *exec.Cmd
	if len(gitBranch) < 2 {
		logrus.Debugf("cloning %q to %q", url, dir)
		cmd = exec.Command("git", "clone", url, dir)
	} else {
		logrus.Debugf("cloning repo %q and branch %q to %q", gitBranch[0], gitBranch[1], dir)
		cmd = exec.Command("git", "clone", "--recurse-submodules", "-b", gitBranch[1], gitBranch[0], dir)
	}
	return cmd.Run()
}

func downloadToDirectory(url, dir string) error {
	logrus.Debugf("extracting %q to %q", url, dir)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength == 0 {
		return errors.Errorf("no contents in %q", url)
	}
	if err := chrootarchive.Untar(resp.Body, dir, nil); err != nil {
		resp1, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp1.Body.Close()
		body, err := ioutil.ReadAll(resp1.Body)
		if err != nil {
			return err
		}
		dockerfile := filepath.Join(dir, "Dockerfile")
		// Assume this is a Dockerfile
		if err := ioutils.AtomicWriteFile(dockerfile, body, 0600); err != nil {
			return errors.Wrapf(err, "Failed to write %q to %q", url, dockerfile)
		}
	}
	return nil
}

func stdinToDirectory(dir string) error {
	logrus.Debugf("extracting stdin to %q", dir)
	r := bufio.NewReader(os.Stdin)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrapf(err, "Failed to read from stdin")
	}
	reader := bytes.NewReader(b)
	if err := chrootarchive.Untar(reader, dir, nil); err != nil {
		dockerfile := filepath.Join(dir, "Dockerfile")
		// Assume this is a Dockerfile
		if err := ioutils.AtomicWriteFile(dockerfile, b, 0600); err != nil {
			return errors.Wrapf(err, "Failed to write bytes to %q", dockerfile)
		}
	}
	return nil
}
