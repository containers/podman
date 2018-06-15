package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Runtime API constants
const (
	// DefaultTransport is a prefix that we apply to an image name
	// to check docker hub first for the image
	DefaultTransport = "docker://"
)

// WriteFile writes a provided string to a provided path
func WriteFile(content string, path string) error {
	baseDir := filepath.Dir(path)
	if baseDir != "" {
		if _, err := os.Stat(baseDir); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(content)
	f.Sync()
	return nil
}

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
func WaitForFile(path string, timeout time.Duration) error {
	done := make(chan struct{})
	chControl := make(chan struct{})
	go func() {
		for {
			select {
			case <-chControl:
				return
			default:
				_, err := os.Stat(path)
				if err == nil {
					close(done)
					return
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		close(chControl)
		return errors.Wrapf(ErrInternal, "timed out waiting for file %s", path)
	}
}

// Create a systemd cgroup at the given path
// Will not error if the path already exists
func createSystemdCgroup(path string) error {
	systemdController, err := cgroups.NewSystemd("/")
	if err != nil {
		return err
	}

	if err := systemdController.Create(path, &spec.LinuxResources{}); err != nil {
		// This is gross, but systemd doesn't hand back full Go
		// errors, so it's all we have
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}

		return errors.Wrapf(err, "error creating cgroup %s", path)
	}

	return nil
}

// Remove a systemd cgroup at the given path
// Will not error if the path does not exist (has already been removed)
func deleteSystemdCgroup(path string) error {
	systemdController, err := cgroups.NewSystemd("/")
	if err != nil {
		return err
	}

	// This seems to not error if the given cgroup doesn't exist
	// All the better for us, since we don't really want to error in
	// that case
	if err := systemdController.Delete(path); err != nil {
		return errors.Wrapf(err, "error deleting cgroup %s", path)
	}

	return nil
}

// Remove a cgroupfs cgroup at the given path
// Will not error if the path does not exist (has already been removed)
func deleteCgroupfsCgroup(path string) error {
	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(path))
	if err != nil && err != cgroups.ErrCgroupDeleted {
		return err
	}

	if err := cgroup.Delete(); err != nil {
		return errors.Wrapf(err, "error deleting cgroup %s", path)
	}

	return nil
}
