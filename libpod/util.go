package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	"github.com/containers/libpod/libpod/define"
	"github.com/fsnotify/fsnotify"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Runtime API constants
const (
	// DefaultTransport is a prefix that we apply to an image name
	// to check docker hub first for the image
	DefaultTransport = "docker://"
)

// FuncTimer helps measure the execution time of a function
// For debug purposes, do not leave in code
// used like defer FuncTimer("foo")
func FuncTimer(funcName string) {
	elapsed := time.Since(time.Now())
	fmt.Printf("%s executed in %d ms\n", funcName, elapsed)
}

// CopyStringStringMap deep copies a map[string]string and returns the result
func CopyStringStringMap(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}

// GetPolicyContext creates a signature policy context for the given signature policy path
func GetPolicyContext(path string) (*signature.PolicyContext, error) {
	policy, err := signature.DefaultPolicy(&types.SystemContext{SignaturePolicyPath: path})
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
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

	timeoutChan := time.After(timeout)

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
