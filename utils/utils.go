package utils

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/storage/pkg/archive"
	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExecCmd executes a command with args and returns its output as a string along
// with an error, if any
func ExecCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("`%v %v` failed: %v %v (%v)", name, strings.Join(args, " "), stderr.String(), stdout.String(), err)
	}

	return stdout.String(), nil
}

// ExecCmdWithStdStreams execute a command with the specified standard streams.
func ExecCmdWithStdStreams(stdin io.Reader, stdout, stderr io.Writer, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("`%v %v` failed: %v", name, strings.Join(args, " "), err)
	}

	return nil
}

// ErrDetach is an error indicating that the user manually detached from the
// container.
var ErrDetach = define.ErrDetach

// CopyDetachable is similar to io.Copy but support a detach key sequence to break out.
func CopyDetachable(dst io.Writer, src io.Reader, keys []byte) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			preservBuf := []byte{}
			for i, key := range keys {
				preservBuf = append(preservBuf, buf[0:nr]...)
				if nr != 1 || buf[0] != key {
					break
				}
				if i == len(keys)-1 {
					return 0, ErrDetach
				}
				nr, er = src.Read(buf)
			}
			var nw int
			var ew error
			if len(preservBuf) > 0 {
				nw, ew = dst.Write(preservBuf)
				nr = len(preservBuf)
			} else {
				nw, ew = dst.Write(buf[0:nr])
			}
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// UntarToFileSystem untars an os.file of a tarball to a destination in the filesystem
func UntarToFileSystem(dest string, tarball *os.File, options *archive.TarOptions) error {
	logrus.Debugf("untarring %s", tarball.Name())
	return archive.Untar(tarball, dest, options)
}

// Creates a new tar file and wrties bytes from io.ReadCloser
func CreateTarFromSrc(source string, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return errors.Wrapf(err, "Could not create tarball file '%s'", dest)
	}
	defer file.Close()
	return TarToFilesystem(source, file)
}

// TarToFilesystem creates a tarball from source and writes to an os.file
// provided
func TarToFilesystem(source string, tarball *os.File) error {
	tb, err := Tar(source)
	if err != nil {
		return err
	}
	_, err = io.Copy(tarball, tb)
	if err != nil {
		return err
	}
	logrus.Debugf("wrote tarball file %s", tarball.Name())
	return nil
}

// Tar creates a tarball from source and returns a readcloser of it
func Tar(source string) (io.ReadCloser, error) {
	logrus.Debugf("creating tarball of %s", source)
	return archive.Tar(source, archive.Uncompressed)
}

// RemoveScientificNotationFromFloat returns a float without any
// scientific notation if the number has any.
// golang does not handle conversion of float64s that have scientific
// notation in them and otherwise stinks.  please replace this if you have
// a better implementation.
func RemoveScientificNotationFromFloat(x float64) (float64, error) {
	bigNum := strconv.FormatFloat(x, 'g', -1, 64)
	breakPoint := strings.IndexAny(bigNum, "Ee")
	if breakPoint > 0 {
		bigNum = bigNum[:breakPoint]
	}
	result, err := strconv.ParseFloat(bigNum, 64)
	if err != nil {
		return x, errors.Wrapf(err, "unable to remove scientific number from calculations")
	}
	return result, nil
}

var (
	runsOnSystemdOnce sync.Once
	runsOnSystemd     bool
)

// RunsOnSystemd returns whether the system is using systemd
func RunsOnSystemd() bool {
	runsOnSystemdOnce.Do(func() {
		initCommand, err := ioutil.ReadFile("/proc/1/comm")
		// On errors, default to systemd
		runsOnSystemd = err != nil || strings.TrimRight(string(initCommand), "\n") == "systemd"
	})
	return runsOnSystemd
}

func moveProcessPIDFileToScope(pidPath, slice, scope string) error {
	data, err := ioutil.ReadFile(pidPath)
	if err != nil {
		// do not raise an error if the file doesn't exist
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot read pid file %s", pidPath)
	}
	pid, err := strconv.ParseUint(string(data), 10, 0)
	if err != nil {
		return errors.Wrapf(err, "cannot parse pid file %s", pidPath)
	}

	return moveProcessToScope(int(pid), slice, scope)
}

func moveProcessToScope(pid int, slice, scope string) error {
	err := RunUnderSystemdScope(int(pid), slice, scope)
	// If the PID is not valid anymore, do not return an error.
	if dbusErr, ok := err.(dbus.Error); ok {
		if dbusErr.Name == "org.freedesktop.DBus.Error.UnixProcessIdUnknown" {
			return nil
		}
	}
	return err
}

// MoveRootlessNetnsSlirpProcessToUserSlice moves the slirp4netns process for the rootless netns
// into a different scope so that systemd does not kill it with a container.
func MoveRootlessNetnsSlirpProcessToUserSlice(pid int) error {
	randBytes := make([]byte, 4)
	_, err := rand.Read(randBytes)
	if err != nil {
		return err
	}
	return moveProcessToScope(pid, "user.slice", fmt.Sprintf("rootless-netns-%x.scope", randBytes))
}

// MovePauseProcessToScope moves the pause process used for rootless mode to keep the namespaces alive to
// a separate scope.
func MovePauseProcessToScope(pausePidPath string) {
	var err error

	for i := 0; i < 10; i++ {
		randBytes := make([]byte, 4)
		_, err = rand.Read(randBytes)
		if err != nil {
			logrus.Errorf("failed to read random bytes: %v", err)
			continue
		}
		err = moveProcessPIDFileToScope(pausePidPath, "user.slice", fmt.Sprintf("podman-pause-%x.scope", randBytes))
		if err == nil {
			return
		}
	}

	if err != nil {
		unified, err2 := cgroups.IsCgroup2UnifiedMode()
		if err2 != nil {
			logrus.Warnf("Failed to detect if running with cgroup unified: %v", err)
		}
		if RunsOnSystemd() && unified {
			logrus.Warnf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		} else {
			logrus.Debugf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		}
	}
}

// CreateSCPCommand takes an existing command, appends the given arguments and returns a configured podman command for image scp
func CreateSCPCommand(cmd *exec.Cmd, command []string) *exec.Cmd {
	cmd.Args = append(cmd.Args, command...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

// LoginUser starts the user process on the host so that image scp can use systemd-run
func LoginUser(user string) (*exec.Cmd, error) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		return nil, err
	}
	machinectl, err := exec.LookPath("machinectl")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(machinectl, "shell", "-q", user+"@.host", sleep, "inf")
	err = cmd.Start()
	return cmd, err
}
