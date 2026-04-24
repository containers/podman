// Pesto client for dynamic port forwarding on a running pasta instance.
//
// Pesto updates pasta's forwarding table via a UNIX domain socket (-c).
// Used by rootless bridge networking: pesto replaces the full table with
// the aggregate ports of all running bridge containers on each change.
//
// Limitations:
//   - Full table replacement only (no incremental add/delete yet)
//   - IPv4 only (netavark DNAT is IPv4; IPv6 bindings cause RST)
//   - TCP and UDP only (SCTP is silently skipped)
//   - Brief forwarding gap during replacement with many containers

package pasta

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

// TODO: When pesto gains --add, --clear, --delete flags, switch from full
// table replacement to incremental updates.

const PestoBinaryName = "pesto"

// PestoSetupPorts adds port forwarding rules for a container to the shared
// pasta instance. allPorts must include ports from all bridge containers
// (including the new one) because pesto replaces the entire table.
func PestoSetupPorts(conf *config.Config, socketPath string, allPorts []types.PortMapping) error {
	if socketPath == "" {
		return errors.New("pesto control socket not available")
	}
	logrus.Debugf("pesto: setting up port forwarding (%d total port mappings)", len(allPorts))
	return pestoReplacePorts(conf, socketPath, allPorts)
}

// PestoTeardownPorts removes a container's port forwarding from the shared
// pasta instance. remainingPorts should include ports from all bridge
// containers EXCEPT the one being torn down.
func PestoTeardownPorts(conf *config.Config, socketPath string, remainingPorts []types.PortMapping) error {
	if socketPath == "" {
		return nil
	}
	logrus.Debugf("pesto: tearing down port forwarding (%d remaining port mappings)", len(remainingPorts))
	return pestoReplacePorts(conf, socketPath, remainingPorts)
}

// pestoReplacePorts invokes pesto to replace the forwarding table on the
// running pasta instance reachable via socketPath. ports contains the full
// set of port mappings that should be active after the call.
func pestoReplacePorts(conf *config.Config, socketPath string, ports []types.PortMapping) error {
	pestoPath, err := conf.FindHelperBinary(PestoBinaryName, true)
	if err != nil {
		return fmt.Errorf("could not find pesto binary: %w", err)
	}

	args := portMappingsToPestoArgs(ports)
	args = append(args, socketPath)

	logrus.Debugf("pesto arguments: %s", strings.Join(args, " "))

	out, err := exec.Command(pestoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pesto failed: %w\noutput: %s", err, string(out))
	}
	if len(out) > 0 {
		logrus.Debugf("pesto output: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// portMappingsToPestoArgs converts PortMappings into pesto CLI arguments.
//
// Pesto only forwards traffic from the host into the rootless netns. This
// does NOT perform DNAT to the container. Netavark handles that inside the
// netns. Therefore each mapping uses HostPort as both source and destination
// (e.g. "-t 0.0.0.0/8080") so traffic arrives at the port netavark expects.
func portMappingsToPestoArgs(ports []types.PortMapping) []string {
	var args []string

	hasTCP := false
	hasUDP := false

	for _, p := range ports {
		// Netavark's DNAT rules use "dnat ip to" which only matches IPv4.
		// Restrict pesto to the correct address family so pasta doesn't
		// accept IPv6 connections that can't be DNAT'd (which causes RST).
		addr := "0.0.0.0/"
		if p.HostIP != "" {
			if strings.Contains(p.HostIP, ":") {
				addr = "[" + p.HostIP + "]/"
			} else {
				addr = p.HostIP + "/"
			}
		}

		for protocol := range strings.SplitSeq(p.Protocol, ",") {
			var flag string
			switch protocol {
			case "tcp":
				flag = "-t"
				hasTCP = true
			case "udp":
				flag = "-u"
				hasUDP = true
			default:
				logrus.Warnf("pesto: unsupported protocol %q, skipping", protocol)
				continue
			}

			portRange := p.Range
			if portRange == 0 {
				portRange = 1
			}

			var arg string
			if portRange == 1 {
				arg = fmt.Sprintf("%s%d", addr, p.HostPort)
			} else {
				arg = fmt.Sprintf("%s%d-%d", addr, p.HostPort, p.HostPort+portRange-1)
			}
			args = append(args, flag, arg)
		}
	}

	if !hasTCP {
		args = append(args, "-t", "none")
	}
	if !hasUDP {
		args = append(args, "-u", "none")
	}

	return args
}
