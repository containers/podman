package libpod

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/utils"
	"github.com/fsnotify/fsnotify"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
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
		defer func() {
			if err := watcher.Close(); err != nil {
				logrus.Errorf("Failed to close fsnotify watcher: %v", err)
			}
		}()
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
				return false, err
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
				return false, err
			}
		case <-timeoutChan:
			return false, fmt.Errorf("timed out waiting for file %s: %w", path, define.ErrInternal)
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
		return fmt.Errorf("pod passed in was nil. Container may not be associated with a pod: %w", define.ErrInvalidArg)
	}

	if ctrPod == "" {
		return fmt.Errorf("container is not a member of any pod: %w", define.ErrInvalidArg)
	}

	if ctrPod != p.ID() {
		return fmt.Errorf("pod passed in is not the pod the container is associated with: %w", define.ErrInvalidArg)
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

func queryPackageVersion(cmdArg ...string) string {
	output := unknownPackage
	if 1 < len(cmdArg) {
		cmd := exec.Command(cmdArg[0], cmdArg[1:]...)
		if outp, err := cmd.Output(); err == nil {
			output = string(outp)
			if cmdArg[0] == "/usr/bin/dpkg" {
				r := strings.Split(output, ": ")
				queryFormat := `${Package}_${Version}_${Architecture}`
				cmd = exec.Command("/usr/bin/dpkg-query", "-f", queryFormat, "-W", r[0])
				if outp, err := cmd.Output(); err == nil {
					output = string(outp)
				}
			}
		}
		if cmdArg[0] == "/sbin/apk" {
			prefix := cmdArg[len(cmdArg)-1] + " is owned by "
			output = strings.Replace(output, prefix, "", 1)
		}
	}
	return strings.Trim(output, "\n")
}

func packageVersion(program string) string { // program is full path
	packagers := [][]string{
		{"/usr/bin/rpm", "-q", "-f"},
		{"/usr/bin/dpkg", "-S"},     // Debian, Ubuntu
		{"/usr/bin/pacman", "-Qo"},  // Arch
		{"/usr/bin/qfile", "-qv"},   // Gentoo (quick)
		{"/usr/bin/equery", "b"},    // Gentoo (slow)
		{"/sbin/apk", "info", "-W"}, // Alpine
	}

	for _, cmd := range packagers {
		cmd = append(cmd, program)
		if out := queryPackageVersion(cmd...); out != unknownPackage {
			return out
		}
	}
	return unknownPackage
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
	def, err := config.Default()
	if err != nil {
		return "", err
	}
	if def.Containers.SeccompProfile != "" {
		return def.Containers.SeccompProfile, nil
	}

	_, err = os.Stat(config.SeccompOverridePath)
	if err == nil {
		return config.SeccompOverridePath, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := os.Stat(config.SeccompDefaultPath); err != nil {
		if !os.IsNotExist(err) {
			return "", err
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
		return fmt.Errorf("accessing dependency container %s state: %w", depCtr.ID(), err)
	}
	if state == define.ContainerStateRemoving {
		return fmt.Errorf("cannot use container %s as a dependency as it is being removed: %w", depCtr.ID(), define.ErrCtrStateInvalid)
	}

	if depCtr.ID() == ctr.ID() {
		return fmt.Errorf("must specify another container: %w", define.ErrInvalidArg)
	}

	if ctr.config.Pod != "" && depCtr.PodID() != ctr.config.Pod {
		return fmt.Errorf("container has joined pod %s and dependency container %s is not a member of the pod: %w", ctr.config.Pod, depCtr.ID(), define.ErrInvalidArg)
	}

	return nil
}

// hijackWriteError writes an error to a hijacked HTTP session.
func hijackWriteError(toWrite error, cid string, terminal bool, httpBuf *bufio.ReadWriter) {
	if toWrite != nil {
		errString := []byte(fmt.Sprintf("Error: %v\n", toWrite))
		if !terminal {
			// We need a header.
			header := makeHTTPAttachHeader(2, uint32(len(errString)))
			if _, err := httpBuf.Write(header); err != nil {
				logrus.Errorf("Writing header for container %s attach connection error: %v", cid, err)
			}
		}
		if _, err := httpBuf.Write(errString); err != nil {
			logrus.Errorf("Writing error to container %s HTTP attach connection: %v", cid, err)
		}
		if err := httpBuf.Flush(); err != nil {
			logrus.Errorf("Flushing HTTP buffer for container %s HTTP attach connection: %v", cid, err)
		}
	}
}

// hijackWriteErrorAndClose writes an error to a hijacked HTTP session and
// closes it. Intended to HTTPAttach function.
// If error is nil, it will not be written; we'll only close the connection.
func hijackWriteErrorAndClose(toWrite error, cid string, terminal bool, httpCon io.Closer, httpBuf *bufio.ReadWriter) {
	hijackWriteError(toWrite, cid, terminal, httpBuf)

	if err := httpCon.Close(); err != nil {
		logrus.Errorf("Closing container %s HTTP attach connection: %v", cid, err)
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

// writeHijackHeader writes a header appropriate for the type of HTTP Hijack
// that occurred in a hijacked HTTP connection used for attach.
func writeHijackHeader(r *http.Request, conn io.Writer) {
	// AttachHeader is the literal header sent for upgraded/hijacked connections for
	// attach, sourced from Docker at:
	// https://raw.githubusercontent.com/moby/moby/b95fad8e51bd064be4f4e58a996924f343846c85/api/server/router/container/container_routes.go
	// Using literally to ensure compatibility with existing clients.
	c := r.Header.Get("Connection")
	proto := r.Header.Get("Upgrade")
	if len(proto) == 0 || !strings.EqualFold(c, "Upgrade") {
		// OK - can't upgrade if not requested or protocol is not specified
		fmt.Fprintf(conn,
			"HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	} else {
		// Upgraded
		fmt.Fprintf(conn,
			"HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n",
			proto)
	}
}

// Convert OCICNI port bindings into Inspect-formatted port bindings.
func makeInspectPortBindings(bindings []types.PortMapping, expose map[uint16][]string) map[string][]define.InspectHostPort {
	portBindings := make(map[string][]define.InspectHostPort)
	for _, port := range bindings {
		protocols := strings.Split(port.Protocol, ",")
		for _, protocol := range protocols {
			for i := uint16(0); i < port.Range; i++ {
				key := fmt.Sprintf("%d/%s", port.ContainerPort+i, protocol)
				hostPorts := portBindings[key]
				hostPorts = append(hostPorts, define.InspectHostPort{
					HostIP:   port.HostIP,
					HostPort: fmt.Sprintf("%d", port.HostPort+i),
				})
				portBindings[key] = hostPorts
			}
		}
	}
	// add exposed ports without host port information to match docker
	for port, protocols := range expose {
		for _, protocol := range protocols {
			key := fmt.Sprintf("%d/%s", port, protocol)
			if _, ok := portBindings[key]; !ok {
				portBindings[key] = nil
			}
		}
	}
	return portBindings
}

// Write a given string to a new file at a given path.
// Will error if a file with the given name already exists.
// Will be chown'd to the UID/GID provided and have the provided SELinux label
// set.
func writeStringToPath(path, contents, mountLabel string, uid, gid int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create %s: %w", path, err)
	}
	defer f.Close()
	if err := f.Chown(uid, gid); err != nil {
		return err
	}

	if _, err := f.WriteString(contents); err != nil {
		return fmt.Errorf("unable to write %s: %w", path, err)
	}
	// Relabel runDirResolv for the container
	if err := label.Relabel(path, mountLabel, false); err != nil {
		return err
	}

	return nil
}
