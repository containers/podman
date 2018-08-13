package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// systemdSliceFromPath makes a new systemd slice under the given parent with
// the given name.
// The parent must be a slice. The name must NOT include ".slice"
func systemdSliceFromPath(parent, name string) (string, error) {
	cgroupPath, err := assembleSystemdCgroupName(parent, name)
	if err != nil {
		return "", err
	}

	logrus.Debugf("Created cgroup path %s for parent %s and name %s", cgroupPath, parent, name)

	if err := makeSystemdCgroup(cgroupPath); err != nil {
		return "", errors.Wrapf(err, "error creating cgroup %s", cgroupPath)
	}

	logrus.Debugf("Created cgroup %s", cgroupPath)

	return cgroupPath, nil
}

// makeSystemdCgroup creates a systemd CGroup at the given location.
func makeSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(SystemdDefaultCgroupParent)
	if err != nil {
		return err
	}

	return controller.Create(path, &spec.LinuxResources{})
}

// deleteSystemdCgroup deletes the systemd cgroup at the given location
func deleteSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(SystemdDefaultCgroupParent)
	if err != nil {
		return err
	}

	return controller.Delete(path)
}

// assembleSystemdCgroupName creates a systemd cgroup path given a base and
// a new component to add.
// The base MUST be systemd slice (end in .slice)
func assembleSystemdCgroupName(baseSlice, newSlice string) (string, error) {
	const sliceSuffix = ".slice"

	if !strings.HasSuffix(baseSlice, sliceSuffix) {
		return "", errors.Wrapf(ErrInvalidArg, "cannot assemble cgroup path with base %q - must end in .slice", baseSlice)
	}

	noSlice := strings.TrimSuffix(baseSlice, sliceSuffix)
	final := fmt.Sprintf("%s/%s-%s%s", baseSlice, noSlice, newSlice, sliceSuffix)

	return final, nil
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
