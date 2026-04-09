//go:build !remote

// Pesto integration for rootless bridge network port forwarding.
//
// A shared pasta instance in the rootless netns (-c pasta.sock) handles
// host-side port forwarding. On container start/stop, pesto replaces
// pasta's forwarding table with the aggregate ports of all running bridge
// containers. Pasta forwards via kernel splice (localhost) or TAP
// (external), preserving source IPs. The container sees the real client's
// address instead of a proxy or bridge gateway address.
//
// Container start:
// - netavark sets up bridge + DNAT
// - pesto updates table
//
// Container stop:
// - pesto updates table without stopped container's ports
// - netavark tears down bridge/DNAT
//
// Limitations:
//   - IPv4 only (netavark DNAT is IPv4; pesto binds 0.0.0.0 by default)
//   - Full table replacement per change (brief gap, possible races)
//   - gatherAllRootlessBridgePorts reads all containers from DB (no locks)

package libpod

import (
	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod/define"
)

// TODO: When pesto gains --add, --clear, --delete flags, switch from full
// table replacement to incremental updates to avoid brief port interruptions
// and reduce overhead with many container and possible race conditions.

func (r *Runtime) pestoSocketPath() string {
	info, err := r.network.RootlessNetnsInfo()
	if err != nil || info == nil {
		return ""
	}
	return info.PestoSocketPath
}

// setupRootlessPortMappingViaPesto configures port forwarding for a rootless
// bridge container by updating the shared pasta instance's forwarding table.
func (r *Runtime) setupRootlessPortMappingViaPesto(ctr *Container) error {
	allPorts, err := r.gatherAllRootlessBridgePorts(ctr, true)
	if err != nil {
		return err
	}
	if len(allPorts) == 0 {
		return nil
	}

	if err := pasta.PestoSetupPorts(r.config, r.pestoSocketPath(), allPorts); err != nil {
		return err
	}
	return nil
}

// teardownRootlessPortMappingViaPesto removes a container's ports from pasta's forwarding table.
func (r *Runtime) teardownRootlessPortMappingViaPesto(ctr *Container) error {
	remainingPorts, err := r.gatherAllRootlessBridgePorts(ctr, false)
	if err != nil {
		return err
	}
	return pasta.PestoTeardownPorts(r.config, r.pestoSocketPath(), remainingPorts)
}

// gatherAllRootlessBridgePorts collects port mappings from all running
// rootless bridge containers. When includeCtr is true, ctr's own ports
// are included; when false they are excluded.
func (r *Runtime) gatherAllRootlessBridgePorts(ctr *Container, includeCtr bool) ([]types.PortMapping, error) {
	var allPorts []types.PortMapping

	ctrs, err := r.state.AllContainers(true)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		if c.ID() == ctr.ID() {
			continue
		}
		if c.state.State != define.ContainerStateRunning {
			continue
		}
		if !c.config.NetMode.IsBridge() {
			continue
		}
		allPorts = append(allPorts, c.convertPortMappings()...)
	}
	if includeCtr {
		allPorts = append(allPorts, ctr.convertPortMappings()...)
	}
	return allPorts, nil
}
