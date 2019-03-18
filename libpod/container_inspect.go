package libpod

import (
	"strings"

	"github.com/containers/libpod/pkg/inspect"
	"github.com/cri-o/ocicni/pkg/ocicni"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func (c *Container) getContainerInspectData(size bool, driverData *inspect.Data) (*inspect.ContainerInspectData, error) {
	config := c.config
	runtimeInfo := c.state
	spec, err := c.specFromState()
	if err != nil {
		return nil, err
	}

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

	var mounts []specs.Mount
	for i, mnt := range spec.Mounts {
		mounts = append(mounts, mnt)
		// We only want to show the name of the named volume in the inspect
		// output, so split the path and get the name out of it.
		if strings.Contains(mnt.Source, c.runtime.config.VolumePath) {
			split := strings.Split(mnt.Source[len(c.runtime.config.VolumePath)+1:], "/")
			mounts[i].Source = split[0]
		}
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
		ExitCommand:     config.ExitCommand,
		Namespace:       config.Namespace,
		Rootfs:          config.Rootfs,
		ResolvConfPath:  resolvPath,
		HostnamePath:    hostnamePath,
		HostsPath:       hostsPath,
		StaticDir:       config.StaticDir,
		LogPath:         config.LogPath,
		ConmonPidFile:   config.ConmonPidFile,
		Name:            config.Name,
		Driver:          driverData.Name,
		MountLabel:      config.MountLabel,
		ProcessLabel:    config.ProcessLabel,
		EffectiveCaps:   spec.Process.Capabilities.Effective,
		BoundingCaps:    spec.Process.Capabilities.Bounding,
		AppArmorProfile: spec.Process.ApparmorProfile,
		ExecIDs:         execIDs,
		GraphDriver:     driverData,
		Mounts:          mounts,
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
		IsInfra: c.IsInfra(),
	}

	if c.config.HealthCheckConfig != nil {
		//	This container has a healthcheck defined in it; we need to add it's state
		healthCheckState, err := c.GetHealthCheckLog()
		if err != nil {
			// An error here is not considered fatal; no health state will be displayed
			logrus.Error(err)
		} else {
			data.State.Healthcheck = healthCheckState
		}
	}

	// Copy port mappings into network settings
	if config.PortMappings != nil {
		data.NetworkSettings.Ports = config.PortMappings
	}

	// Get information on the container's network namespace (if present)
	data = c.getContainerNetworkInfo(data)

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
