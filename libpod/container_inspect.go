package libpod

import (
	"strconv"
	"strings"

	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
)

func (c *Container) getContainerInspectData(size bool, driverData *inspect.Data) (*inspect.ContainerInspectData, error) {
	config := c.config
	runtimeInfo := c.state
	spec := c.config.Spec

	// Process is allowed to be nil in the spec
	args := []string{}
	if config.Spec.Process != nil {
		args = config.Spec.Process.Args
	}
	var path string
	if len(args) > 0 {
		path = args[0]
	}
	if len(args) > 1 {
		args = args[1:]
	}

	execIDs := []string{}
	for id := range c.state.ExecSessions {
		execIDs = append(execIDs, id)
	}

	if c.state.BindMounts == nil {
		c.state.BindMounts = make(map[string]string)
	}

	resolvPath := ""
	if getPath, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
		resolvPath = getPath
	}

	hostsPath := ""
	if getPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
		hostsPath = getPath
	}

	hostnamePath := ""
	if getPath, ok := c.state.BindMounts["/etc/hostname"]; ok {
		hostnamePath = getPath
	}

	data := &inspect.ContainerInspectData{
		ID:      config.ID,
		Created: config.CreatedTime,
		Path:    path,
		Args:    args,
		State: &inspect.ContainerInspectState{
			OciVersion: spec.Version,
			Status:     runtimeInfo.State.String(),
			Running:    runtimeInfo.State == ContainerStateRunning,
			Paused:     runtimeInfo.State == ContainerStatePaused,
			OOMKilled:  runtimeInfo.OOMKilled,
			Dead:       runtimeInfo.State.String() == "bad state",
			Pid:        runtimeInfo.PID,
			ExitCode:   runtimeInfo.ExitCode,
			Error:      "", // can't get yet
			StartedAt:  runtimeInfo.StartedTime,
			FinishedAt: runtimeInfo.FinishedTime,
		},
		ImageID:         config.RootfsImageID,
		ImageName:       config.RootfsImageName,
		Rootfs:          config.Rootfs,
		ResolvConfPath:  resolvPath,
		HostnamePath:    hostnamePath,
		HostsPath:       hostsPath,
		StaticDir:       config.StaticDir,
		LogPath:         config.LogPath,
		Name:            config.Name,
		Driver:          driverData.Name,
		MountLabel:      config.MountLabel,
		ProcessLabel:    spec.Process.SelinuxLabel,
		AppArmorProfile: spec.Process.ApparmorProfile,
		ExecIDs:         execIDs,
		GraphDriver:     driverData,
		Mounts:          spec.Mounts,
		Dependencies:    c.Dependencies(),
		NetworkSettings: &inspect.NetworkSettings{
			Bridge:                 "",    // TODO
			SandboxID:              "",    // TODO - is this even relevant?
			HairpinMode:            false, // TODO
			LinkLocalIPv6Address:   "",    // TODO - do we even support IPv6?
			LinkLocalIPv6PrefixLen: 0,     // TODO - do we even support IPv6?
			Ports:                  []ocicni.PortMapping{}, // TODO - maybe worth it to put this in Docker format?
			SandboxKey:             "",                     // Network namespace path
			SecondaryIPAddresses:   nil,                    // TODO - do we support this?
			SecondaryIPv6Addresses: nil,                    // TODO - do we support this?
			EndpointID:             "",                     // TODO - is this even relevant?
			Gateway:                "",                     // TODO
			GlobalIPv6Address:      "",
			GlobalIPv6PrefixLen:    0,
			IPAddress:              "",
			IPPrefixLen:            0,
			IPv6Gateway:            "",
			MacAddress:             "", // TODO
		},
	}

	// Copy port mappings into network settings
	if config.PortMappings != nil {
		data.NetworkSettings.Ports = config.PortMappings
	}

	// Get information on the container's network namespace (if present)
	if runtimeInfo.NetNS != nil {
		// Go through our IP addresses
		for _, ctrIP := range c.state.IPs {
			ipWithMask := ctrIP.Address.String()
			splitIP := strings.Split(ipWithMask, "/")
			mask, _ := strconv.Atoi(splitIP[1])
			if ctrIP.Version == "4" {
				data.NetworkSettings.IPAddress = splitIP[0]
				data.NetworkSettings.IPPrefixLen = mask
				data.NetworkSettings.Gateway = ctrIP.Gateway.String()
			} else {
				data.NetworkSettings.GlobalIPv6Address = splitIP[0]
				data.NetworkSettings.GlobalIPv6PrefixLen = mask
				data.NetworkSettings.IPv6Gateway = ctrIP.Gateway.String()
			}
		}

		// Set network namespace path
		data.NetworkSettings.SandboxKey = runtimeInfo.NetNS.Path()

		// Set MAC address of interface linked with network namespace path
		for _, i := range c.state.Interfaces {
			if i.Sandbox == data.NetworkSettings.SandboxKey {
				data.NetworkSettings.MacAddress = i.Mac
			}
		}
	}

	if size {
		rootFsSize, err := c.rootFsSize()
		if err != nil {
			logrus.Errorf("error getting rootfs size %q: %v", config.ID, err)
		}
		rwSize, err := c.rwSize()
		if err != nil {
			logrus.Errorf("error getting rw size %q: %v", config.ID, err)
		}
		data.SizeRootFs = rootFsSize
		data.SizeRw = rwSize
	}
	return data, nil
}
