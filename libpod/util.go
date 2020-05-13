package libpod

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/utils"
	"github.com/fsnotify/fsnotify"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Runtime API constants
const (
	unknownPackage = "Unknown"
)

// FuncTimer helps measure the execution time of a function
// For debug purposes, do not leave in code
// used like defer FuncTimer("foo")
func FuncTimer(funcName string) {
	elapsed := time.Since(time.Now())
	fmt.Printf("%s executed in %d ms\n", funcName, elapsed)
}

// MountExists returns true if dest exists in the list of mounts
func MountExists(specMounts []spec.Mount, dest string) bool {
	for _, m := range specMounts {
		if m.Destination == dest {
			return true
		}
	}
	return false
}

// WaitForFile waits until a file has been created or the given timeout has occurred
func WaitForFile(path string, chWait chan error, timeout time.Duration) (bool, error) {
	var inotifyEvents chan fsnotify.Event
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		if err := watcher.Add(filepath.Dir(path)); err == nil {
			inotifyEvents = watcher.Events
		}
		defer watcher.Close()
	}

	var timeoutChan <-chan time.Time

	if timeout != 0 {
		timeoutChan = time.After(timeout)
	}

	for {
		select {
		case e := <-chWait:
			return true, e
		case <-inotifyEvents:
			_, err := os.Stat(path)
			if err == nil {
				return false, nil
			}
			if !os.IsNotExist(err) {
				return false, errors.Wrapf(err, "checking file %s", path)
			}
		case <-time.After(25 * time.Millisecond):
			// Check periodically for the file existence.  It is needed
			// if the inotify watcher could not have been created.  It is
			// also useful when using inotify as if for any reasons we missed
			// a notification, we won't hang the process.
			_, err := os.Stat(path)
			if err == nil {
				return false, nil
			}
			if !os.IsNotExist(err) {
				return false, errors.Wrapf(err, "checking file %s", path)
			}
		case <-timeoutChan:
			return false, errors.Wrapf(define.ErrInternal, "timed out waiting for file %s", path)
		}
	}
}

type byDestination []spec.Mount

func (m byDestination) Len() int {
	return len(m)
}

func (m byDestination) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

func (m byDestination) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m byDestination) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

func sortMounts(m []spec.Mount) []spec.Mount {
	sort.Sort(byDestination(m))
	return m
}

func validPodNSOption(p *Pod, ctrPod string) error {
	if p == nil {
		return errors.Wrapf(define.ErrInvalidArg, "pod passed in was nil. Container may not be associated with a pod")
	}

	if ctrPod == "" {
		return errors.Wrapf(define.ErrInvalidArg, "container is not a member of any pod")
	}

	if ctrPod != p.ID() {
		return errors.Wrapf(define.ErrInvalidArg, "pod passed in is not the pod the container is associated with")
	}
	return nil
}

// JSONDeepCopy performs a deep copy by performing a JSON encode/decode of the
// given structures. From and To should be identically typed structs.
func JSONDeepCopy(from, to interface{}) error {
	tmp, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(tmp, to)
}

func dpkgVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/dpkg", "-S", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func rpmVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/rpm", "-q", "-f", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func packageVersion(program string) string {
	if out := rpmVersion(program); out != unknownPackage {
		return out
	}
	return dpkgVersion(program)
}

func programVersion(mountProgram string) (string, error) {
	output, err := utils.ExecCmd(mountProgram, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(output, "\n"), nil
}

// DefaultSeccompPath returns the path to the default seccomp.json file
// if it exists, first it checks OverrideSeccomp and then default.
// If neither exist function returns ""
func DefaultSeccompPath() (string, error) {
	_, err := os.Stat(config.SeccompOverridePath)
	if err == nil {
		return config.SeccompOverridePath, nil
	}
	if !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "can't check if %q exists", config.SeccompOverridePath)
	}
	if _, err := os.Stat(config.SeccompDefaultPath); err != nil {
		if !os.IsNotExist(err) {
			return "", errors.Wrapf(err, "can't check if %q exists", config.SeccompDefaultPath)
		}
		return "", nil
	}
	return config.SeccompDefaultPath, nil
}

// CheckDependencyContainer verifies the given container can be used as a
// dependency of another container.
// Both the dependency to check and the container that will be using the
// dependency must be passed in.
// It is assumed that ctr is locked, and depCtr is unlocked.
func checkDependencyContainer(depCtr, ctr *Container) error {
	state, err := depCtr.State()
	if err != nil {
		return errors.Wrapf(err, "error accessing dependency container %s state", depCtr.ID())
	}
	if state == define.ContainerStateRemoving {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot use container %s as a dependency as it is being removed", depCtr.ID())
	}

	if depCtr.ID() == ctr.ID() {
		return errors.Wrapf(define.ErrInvalidArg, "must specify another container")
	}

	if ctr.config.Pod != "" && depCtr.PodID() != ctr.config.Pod {
		return errors.Wrapf(define.ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, depCtr.ID())
	}

	return nil
}

// hijackWriteErrorAndClose writes an error to a hijacked HTTP session and
// closes it. Intended to HTTPAttach function.
// If error is nil, it will not be written; we'll only close the connection.
func hijackWriteErrorAndClose(toWrite error, cid string, terminal bool, httpCon io.Closer, httpBuf *bufio.ReadWriter) {
	if toWrite != nil {
		errString := []byte(fmt.Sprintf("%v\n", toWrite))
		if !terminal {
			// We need a header.
			header := makeHTTPAttachHeader(2, uint32(len(errString)))
			if _, err := httpBuf.Write(header); err != nil {
				logrus.Errorf("Error writing header for container %s attach connection error: %v", cid, err)
			}
			// TODO: May want to return immediately here to avoid
			// writing garbage to the socket?
		}
		if _, err := httpBuf.Write(errString); err != nil {
			logrus.Errorf("Error writing error to container %s HTTP attach connection: %v", cid, err)
		}
		if err := httpBuf.Flush(); err != nil {
			logrus.Errorf("Error flushing HTTP buffer for container %s HTTP attach connection: %v", cid, err)
		}
	}

	if err := httpCon.Close(); err != nil {
		logrus.Errorf("Error closing container %s HTTP attach connection: %v", cid, err)
	}
}

// makeHTTPAttachHeader makes an 8-byte HTTP header for a buffer of the given
// length and stream. Accepts an integer indicating which stream we are sending
// to (STDIN = 0, STDOUT = 1, STDERR = 2).
func makeHTTPAttachHeader(stream byte, length uint32) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], length)
	return header
}
