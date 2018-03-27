package libpod

import (
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
)

func (c *Container) getContainerInspectData(size bool, driverData *inspect.Data) (*inspect.ContainerInspectData, error) {
	config := c.config
	runtimeInfo := c.state
	spec := c.config.Spec

	args := config.Spec.Process.Args
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
			GlobalIPv6Addresses:    []string{},             // TODO - do we even support IPv6?
			GlobalIPv6PrefixLen:    0,                      // TODO - do we even support IPv6?
			IPAddress:              "",
			IPPrefixLen:            0,  // TODO
			IPv6Gateway:            "", // TODO - do we even support IPv6?
			MacAddress:             "", // TODO
		},
	}

	// Copy port mappings into network settings
	if config.PortMappings != nil {
		data.NetworkSettings.Ports = config.PortMappings
	}

	// Get information on the container's network namespace (if present)
	if runtimeInfo.NetNS != nil {
		// Get IP address
		ip, err := c.runtime.getContainerIP(c)
		if err != nil {
			logrus.Errorf("error getting container %q IP: %v", config.ID, err)
		} else {
			data.NetworkSettings.IPAddress = ip.To4().String()
		}

		// Set network namespace path
		data.NetworkSettings.SandboxKey = runtimeInfo.NetNS.Path()
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
