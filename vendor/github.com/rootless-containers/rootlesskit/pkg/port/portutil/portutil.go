package portutil

import (
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/port"
)

// ParsePortSpec parses a Docker-like representation of PortSpec.
// e.g. "127.0.0.1:8080:80/tcp", or "127.0.0.1:8080:10.0.2.100:80/tcp"
func ParsePortSpec(s string) (*port.Spec, error) {
	splitBySlash := strings.SplitN(s, "/", 2)
	if len(splitBySlash) != 2 {
		return nil, errors.Errorf("unexpected PortSpec string: %q", s)
	}
	proto := splitBySlash[1]
	switch proto {
	case "tcp", "udp", "sctp":
	default:
		return nil, errors.Errorf("unexpected Proto in PortSpec string: %q", s)
	}

	splitByColon := strings.SplitN(splitBySlash[0], ":", 4)
	switch len(splitByColon) {
	case 3, 4:
	default:
		return nil, errors.Errorf("unexpected PortSpec string: %q", s)
	}

	parentIP := splitByColon[0]
	if net.IP(parentIP) == nil {
		return nil, errors.Errorf("unexpected ParentIP in PortSpec string: %q", s)
	}

	parentPort, err := strconv.Atoi(splitByColon[1])
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected ParentPort in PortSpec string: %q", s)
	}

	var childIP string
	if len(splitByColon) == 4 {
		childIP = splitByColon[2]
		if net.IP(childIP) == nil {
			return nil, errors.Errorf("unexpected ChildIP in PortSpec string: %q", s)
		}
	}

	childPort, err := strconv.Atoi(splitByColon[len(splitByColon)-1])
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected ChildPort in PortSpec string: %q", s)
	}

	return &port.Spec{
		Proto:      proto,
		ParentIP:   parentIP,
		ParentPort: parentPort,
		ChildIP:    childIP,
		ChildPort:  childPort,
	}, nil
}

// ValidatePortSpec validates *port.Spec.
// existingPorts can be optionally passed for detecting conflicts.
func ValidatePortSpec(spec port.Spec, existingPorts map[int]*port.Status) error {
	if spec.Proto != "tcp" && spec.Proto != "udp" {
		return errors.Errorf("unknown proto: %q", spec.Proto)
	}
	if spec.ParentIP != "" {
		if net.ParseIP(spec.ParentIP) == nil {
			return errors.Errorf("invalid ParentIP: %q", spec.ParentIP)
		}
	}
	if spec.ParentPort <= 0 || spec.ParentPort > 65535 {
		return errors.Errorf("invalid ParentPort: %q", spec.ParentPort)
	}
	if spec.ChildPort <= 0 || spec.ChildPort > 65535 {
		return errors.Errorf("invalid ChildPort: %q", spec.ChildPort)
	}
	for id, p := range existingPorts {
		sp := p.Spec
		sameProto := sp.Proto == spec.Proto
		sameParent := sp.ParentIP == spec.ParentIP && sp.ParentPort == spec.ParentPort
		if sameProto && sameParent {
			return errors.Errorf("conflict with ID %d", id)
		}
	}
	return nil
}
