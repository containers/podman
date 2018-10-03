package port

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrInvalidArg is a badly-formatted port being passed.
	ErrInvalidArg = errors.Errorf("invalid port specified")
	// ErrRangeConflict indicates a conflict between the port being added
	// and an existing port mapping.
	ErrRangeConflict = errors.Errorf("conflict between requested port mappings")
	// ErrInternal indicates an error in internal package logic has occurred
	ErrInternal = errors.Errorf("internal error creating port range")
)

// AddPortToMapping adds a port to a set of existing port mappings.
// Handles overlap by adding the port to the existing range.
// Validates the newest mapping to be added.
func AddPortToMapping(newPort PortMapping, ports []PortMapping) ([]PortMapping, error) {
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
	if int32(newPort.Length-1)+newPort.ContainerPort > 65535 {
		return nil, errors.Wrapf(ErrInvalidArg, "port range exceeds maximum allowable port number in container")
	}
	if int32(newPort.Length-1)+newPort.HostPort > 65535 {
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
				logrus.Debugf("Port range (protocol %s) starting at %d with length %d contained entirely within port range starting at %d length %d, ignoring",
					port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)

				// We're done - no changes need to be made to the old mappings
				return ports, nil
			}

			// There's a mismatch in the mappings - error out
			return nil, errors.Wrapf(ErrRangeConflict, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d",
				port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)
		}

		// Does the new port's container port entirely contain the range
		// of the old port?
		if checkInRange(port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length) {
			// Does the host port match as well?
			// Need to check the difference between host port and
			// container port to make sure the mappings match.
			if checkInRange(port.HostPort, port.Length, newPort.HostPort, newPort.Length) &&
				(newPort.ContainerPort-newPort.HostPort) == (port.ContainerPort-port.HostPort) {
				logrus.Debugf("Port range (protocol %s) starting at %d with length %d contained entirely within port range starting at %d length %d, updating old mapping",
					port.Protocol, port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length)

				// Decline to include the old port as it is
				// entirely within the new port's range.
				// Keep iterating so we can slurp up similar
				// ports.
				continue
			}

			// There's a mismatch in the mappings, error out
			return nil, errors.Wrapf(ErrRangeConflict, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d",
				port.Protocol, port.ContainerPort, port.Length, newPort.ContainerPort, newPort.Length)
		}

		// Is there an overlap in the port ranges?
		if checkRangeOverlap(newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length) {
			// If the host port ranges don't match, we have a problem
			if !(checkRangeOverlap(newPort.HostPort, newPort.Length, port.HostPort, port.Length) &&
				(newPort.ContainerPort-newPort.HostPort) == (port.ContainerPort-port.HostPort)) {
				return nil, errors.Wrapf(ErrRangeConflict, "port range (protocol %s) starting at %d with length %d has host port mismatch with port range starting at %d with length %d",
					port.Protocol, newPort.ContainerPort, newPort.Length, port.ContainerPort, port.Length)
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

// Check if there is a range overlap.
// Allows for ranges that are directly adjacent but do not overlap - that is,
// ranges 80-82 and 83-84 will still match
func checkRangeOverlap(toCheck int32, checkRange uint16, rangeStart int32, rangeLength uint16) bool {
	// Normally we would subtract 1 here, because range lengths begin at 1
	// for a single port, and the end of that range would be the same port.
	// However, because we allow directly adjacent but not overlapping
	// ranges, we allow the ranges to end 1 higher than they normally would.
	toCheckEnd := toCheck + int32(checkRange)
	rangeEnd := rangeStart + int32(rangeLength)

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
