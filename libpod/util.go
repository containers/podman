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

// Add a port to a sec port port mappings.
// Handles overlap by adding the port to the existing range.
// Validates the newest mapping to be added.
func addPortToMapping(newPort PortMapping, ports []PortMapping) ([]PortMapping, error) {
	// First, validate the port mapping
	if newPort.ContainerPort < 0 || newPort.ContainerPort > 65535 {
		return nil, errors.Wrapf(ErrInvalidArg, "container port must be between 0 and 65535, instead got %d", newPort.ContainerPort)
	}
	if newPort.HostPort < 0 || newPort.HostPort > 65535 {
		return nil, errors.Wrapf(ErrInvalidArg, "host port must be between 0 and 65535, instead got %d", newPort.HostPort)
	}
	// Length of 0 is invalid
	if newPort.Length == 0 {
		return nil, errors.Wrapf(ErrInvalidArg, "length of a port mapping must be greater than 0")
	}
	// Since length is 1 for a single port, we want to subtract one to get an
	// accurate measure of the ending port.
	if int32(newPort.Length-1) + newPort.ContainerPort > 65535 {
		return nil, errors.Wrapf(ErrInvalidArg, "port range exceeds maximum allowable port number in container")
	}
	if int32(newPort.Length-1) + newPort.HostPort > 65535 {
		return nil, errors.Wrapf(ErrInvalidArg, "port range exceeds maximum allowable port number on host")
	}
	if newPort.Protocol != "tcp" && newPort.Protocol != "udp" {
		return nil, errors.Wrapf(ErrInvalidArg, "protocol %q is not valid (available protocols tcp and udp)", newPort.Protocol)
	}

	// If we are the first port mapping, add the port mapping
	if len(ports) == 0 {
		ports = append(ports, newPort)
		return ports, nil
	}

	newPorts := make([]PortMapping, 0, len(ports)+1)

	// Alright, we have existing ports.
	// Let's see if this can be added to any existing port mapping.
	for _, port := range ports {
		// If protocol or host IP don't match, just continue
		// TODO: More in-depth host IP handling might be desirable -
		// does forwarding the same port to 0.0.0.0 and another IP make
		// sense?
		if newPort.Protocol != port.Protocol || newPort.HostIP != port.HostIP {
			continue
		}

			// Is the new port's container port entirely within the range of
		// the old port?
		if checkInRange(newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length) {
			// Does the host port match as well?
			// Need to check the difference between host port and
			// container port to make sure the mappings match.
			if checkInRange(newPort.HostPort, newPort.Length, port.HostPort, port.Length) &&
				(newPort.ContainerPort-newPort.HostPort) == (port.ContainerPort-port.HostPort) {
				logrus.Debugf("Port range (protocol %s) starting at %d with length %d contained entirely within port range starting at %d length %d, ignoring", port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)

				// We're done - no changes need to be made to the old mappings
				return ports, nil
			}

			// There's a mismatch in the mappings - error out
			return nil, errors.Wrapf(ErrInvalidArg, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d", port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)
		}

		// Does the new port's container port entirely contain the range
		// of the old port?
		if checkInRange(port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length) {
			// Does the host port match as well?
			// Need to check the difference between host port and
			// container port to make sure the mappings match.
			if checkInRange(port.HostPort, port.Length, newPort.HostPort, newPort.Length) &&
				(newPort.ContainerPort-newPort.HostPort) == (port.ContainerPort-port.HostPort) {
				logrus.Debugf("Port range (protocol %s) starting at %d with length %d contained entirely within port range starting at %d length %d, updating old mapping", port.Protocol, port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length)

				// Decline to include the old port as it is
				// entirely within the new port's range.
				// Keep iterating so we can slurp up similar
				// ports.
				continue
			}

			// There's a mismatch in the mappings, error out
			return nil, errors.Wrapf(ErrInvalidArg, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d", port.Protocol, port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length)
		}

		// Is there an overlap in the port ranges?
		if checkRangeOverlap(newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length) {
			// If the host port ranges don't match, we have a problem
			if !(checkRangeOverlap(newPort.HostPort, newPort.Length, port.HostPort, port.Length) &&
				(newPort.ContainerPort-newPort.HostPort) == (port.ContainerPort-port.HostPort)) {
				return nil, errors.Wrapf(ErrInvalidArg, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d", port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)
			}

			// Build one port mapping out of the two overlapping mappings
			startPortCtr := port.ContainerPort
			if newPort.ContainerPort < startPortCtr {
				startPortCtr = newPort.ContainerPort
			}

			startPortHost := port.HostPort
			if newPort.HostPort < startPortHost {
				startPortHost = newPort.HostPort
			}

			endPort := port.HostPort + int32(port.Length)
			endPortNew := newPort.HostPort + int32(newPort.Length)
			if endPortNew > endPort {
				endPort = endPortNew
			}

			length := endPort - startPortHost + 1
			if length < 0 || length > 65535 {
				// Something has gone seriously wrong
				return nil, errors.Wrapf(ErrInternal, "attemped to create port range of length %d from start %d", length, startPortCtr)
			}

			portStruct := PortMapping{
				ContainerPort: startPortCtr,
				HostPort:      startPortHost,
				HostIP:        port.HostIP,
				Length:        uint16(length),
				Protocol:      port.Protocol,
			}

			// Replace the port we've been testing with with the new
			// port mapping we made
			newPort = portStruct

			// And move on to test the rest of our mappings
			continue
		}

		// The port didn't match.
		// Include it.
		newPorts = append(newPorts, port)
	}

	// Didn't find a match
	// Append the port to the mappings and return
	newPorts = append(newPorts, newPort)
	return newPorts, nil
}

// Check if a given port is in the given range
func checkInRange(toCheck int32, checkRange uint16, rangeStart int32, rangeLength uint16) bool {
	toCheckEnd := toCheck + int32(checkRange-1)
	rangeEnd := rangeStart + int32(rangeLength-1)

	return (toCheck <= rangeEnd && toCheck >= rangeStart) &&
		(toCheckEnd <= rangeEnd && toCheckEnd >= rangeStart)
}

// Check if there is a range overlap
func checkRangeOverlap(toCheck int32, checkRange uint16, rangeStart int32, rangeLength uint16) bool {
	toCheckEnd := toCheck + int32(checkRange-1)
	rangeEnd := rangeStart + int32(rangeLength-1)

	if toCheck <= rangeEnd && toCheck >= rangeStart {
		return true
	}
	if toCheckEnd <= rangeEnd && toCheckEnd >= rangeStart {
		return true
	}
	if rangeStart <= toCheckEnd && rangeStart >= toCheck {
		return true
	}
	if rangeEnd <= toCheckEnd && rangeEnd >= toCheck {
		return true
	}

	return false
}
