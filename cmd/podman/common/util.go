package common

import (
	"net"
	"strconv"
	"strings"

	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// createExpose parses user-provided exposed port definitions and converts them
// into SpecGen format.
// TODO: The SpecGen format should really handle ranges more sanely - we could
// be massively inflating what is sent over the wire with a large range.
func createExpose(expose []string) (map[uint16]string, error) {
	toReturn := make(map[uint16]string)

	for _, e := range expose {
		// Check for protocol
		proto := "tcp"
		splitProto := strings.Split(e, "/")
		if len(splitProto) > 2 {
			return nil, errors.Errorf("invalid expose format - protocol can only be specified once")
		} else if len(splitProto) == 2 {
			proto = splitProto[1]
		}

		// Check for a range
		start, len, err := parseAndValidateRange(splitProto[0])
		if err != nil {
			return nil, err
		}

		var index uint16
		for index = 0; index < len; index++ {
			portNum := start + index
			protocols, ok := toReturn[portNum]
			if !ok {
				toReturn[portNum] = proto
			} else {
				newProto := strings.Join(append(strings.Split(protocols, ","), strings.Split(proto, ",")...), ",")
				toReturn[portNum] = newProto
			}
		}
	}

	return toReturn, nil
}

// createPortBindings iterates ports mappings into SpecGen format.
func createPortBindings(ports []string) ([]specgen.PortMapping, error) {
	// --publish is formatted as follows:
	// [[hostip:]hostport[-endPort]:]containerport[-endPort][/protocol]
	toReturn := make([]specgen.PortMapping, 0, len(ports))

	for _, p := range ports {
		var (
			ctrPort                 string
			proto, hostIP, hostPort *string
		)

		splitProto := strings.Split(p, "/")
		switch len(splitProto) {
		case 1:
			// No protocol was provided
		case 2:
			proto = &(splitProto[1])
		default:
			return nil, errors.Errorf("invalid port format - protocol can only be specified once")
		}

		splitPort := strings.Split(splitProto[0], ":")
		switch len(splitPort) {
		case 1:
			ctrPort = splitPort[0]
		case 2:
			hostPort = &(splitPort[0])
			ctrPort = splitPort[1]
		case 3:
			hostIP = &(splitPort[0])
			hostPort = &(splitPort[1])
			ctrPort = splitPort[2]
		default:
			return nil, errors.Errorf("invalid port format - format is [[hostIP:]hostPort:]containerPort")
		}

		newPort, err := parseSplitPort(hostIP, hostPort, ctrPort, proto)
		if err != nil {
			return nil, err
		}

		toReturn = append(toReturn, newPort)
	}

	return toReturn, nil
}

// parseSplitPort parses individual components of the --publish flag to produce
// a single port mapping in SpecGen format.
func parseSplitPort(hostIP, hostPort *string, ctrPort string, protocol *string) (specgen.PortMapping, error) {
	newPort := specgen.PortMapping{}
	if ctrPort == "" {
		return newPort, errors.Errorf("must provide a non-empty container port to publish")
	}
	ctrStart, ctrLen, err := parseAndValidateRange(ctrPort)
	if err != nil {
		return newPort, errors.Wrapf(err, "error parsing container port")
	}
	newPort.ContainerPort = ctrStart
	newPort.Range = ctrLen

	if protocol != nil {
		if *protocol == "" {
			return newPort, errors.Errorf("must provide a non-empty protocol to publish")
		}
		newPort.Protocol = *protocol
	}
	if hostIP != nil {
		if *hostIP == "" {
			return newPort, errors.Errorf("must provide a non-empty container host IP to publish")
		}
		testIP := net.ParseIP(*hostIP)
		if testIP == nil {
			return newPort, errors.Errorf("cannot parse %q as an IP address", *hostIP)
		}
		newPort.HostIP = testIP.String()
	}
	if hostPort != nil {
		if *hostPort == "" {
			return newPort, errors.Errorf("must provide a non-empty container host port to publish")
		}
		hostStart, hostLen, err := parseAndValidateRange(*hostPort)
		if err != nil {
			return newPort, errors.Wrapf(err, "error parsing host port")
		}
		if hostLen != ctrLen {
			return newPort, errors.Errorf("host and container port ranges have different lengths: %d vs %d", hostLen, ctrLen)
		}
		newPort.HostPort = hostStart
	}

	hport := newPort.HostPort
	if hport == 0 {
		hport = newPort.ContainerPort
	}
	logrus.Debugf("Adding port mapping from %d to %d length %d protocol %q", hport, newPort.ContainerPort, newPort.Range, newPort.Protocol)

	return newPort, nil
}

// Parse and validate a port range.
// Returns start port, length of range, error.
func parseAndValidateRange(portRange string) (uint16, uint16, error) {
	splitRange := strings.Split(portRange, "-")
	if len(splitRange) > 2 {
		return 0, 0, errors.Errorf("invalid port format - port ranges are formatted as startPort-stopPort")
	}

	if splitRange[0] == "" {
		return 0, 0, errors.Errorf("port numbers cannot be negative")
	}

	startPort, err := parseAndValidatePort(splitRange[0])
	if err != nil {
		return 0, 0, err
	}

	var rangeLen uint16 = 1
	if len(splitRange) == 2 {
		if splitRange[1] == "" {
			return 0, 0, errors.Errorf("must provide ending number for port range")
		}
		endPort, err := parseAndValidatePort(splitRange[1])
		if err != nil {
			return 0, 0, err
		}
		if endPort <= startPort {
			return 0, 0, errors.Errorf("the end port of a range must be higher than the start port - %d is not higher than %d", endPort, startPort)
		}
		// Our range is the total number of ports
		// involved, so we need to add 1 (8080:8081 is
		// 2 ports, for example, not 1)
		rangeLen = endPort - startPort + 1
	}

	return startPort, rangeLen, nil
}

// Turn a single string into a valid U16 port.
func parseAndValidatePort(port string) (uint16, error) {
	num, err := strconv.Atoi(port)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot parse %q as a port number", port)
	}
	if num < 1 || num > 65535 {
		return 0, errors.Errorf("port numbers must be between 1 and 65535 (inclusive), got %d", num)
	}
	return uint16(num), nil
}
