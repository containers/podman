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
	"github.com/containers/libpod/pkg/util"
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
		return errors.Wrapf(ErrInvalidArg, "pod passed in was nil. Container may not be associated with a pod")
	}

	if ctrPod == "" {
		return errors.Wrapf(ErrInvalidArg, "container is not a member of any pod")
	}

	if ctrPod != p.ID() {
		return errors.Wrapf(ErrInvalidArg, "pod passed in is not the pod the container is associated with")
	}
	return nil
}

// GetV1CGroups gets the V1 cgroup subsystems and then "filters"
// out any subsystems that are provided by the caller.  Passing nil
// for excludes will return the subsystems unfiltered.
//func GetV1CGroups(excludes []string) ([]cgroups.Subsystem, error) {
func GetV1CGroups(excludes []string) cgroups.Hierarchy {
	return func() ([]cgroups.Subsystem, error) {
		var filtered []cgroups.Subsystem

		subSystem, err := cgroups.V1()
		if err != nil {
			return nil, err
		}
		for _, s := range subSystem {
			// If the name of the subsystem is not in the list of excludes, then
			// add it as a keeper.
			if !util.StringInSlice(string(s.Name()), excludes) {
				filtered = append(filtered, s)
			}
		}
		return filtered, nil
	}
}
