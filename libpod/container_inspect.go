package libpod

import (
	"strings"
	"time"

	"github.com/containers/libpod/libpod/driver"
	"github.com/cri-o/ocicni/pkg/ocicni"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// InspectContainerData provides a detailed record of a container's configuration
// and state as viewed by Libpod.
// Large portions of this structure are defined such that the output is
// compatible with `docker inspect` JSON, but additional fields have been added
// as required to share information not in the original output.
type InspectContainerData struct {
	ID              string                  `json:"Id"`
	Created         time.Time               `json:"Created"`
	Path            string                  `json:"Path"`
	Args            []string                `json:"Args"`
	State           *InspectContainerState  `json:"State"`
	ImageID         string                  `json:"Image"`
	ImageName       string                  `json:"ImageName"`
	Rootfs          string                  `json:"Rootfs"`
	ResolvConfPath  string                  `json:"ResolvConfPath"`
	HostnamePath    string                  `json:"HostnamePath"`
	HostsPath       string                  `json:"HostsPath"`
	StaticDir       string                  `json:"StaticDir"`
	OCIConfigPath   string                  `json:"OCIConfigPath,omitempty"`
	LogPath         string                  `json:"LogPath"`
	ConmonPidFile   string                  `json:"ConmonPidFile"`
	Name            string                  `json:"Name"`
	RestartCount    int32                   `json:"RestartCount"`
	Driver          string                  `json:"Driver"`
	MountLabel      string                  `json:"MountLabel"`
	ProcessLabel    string                  `json:"ProcessLabel"`
	AppArmorProfile string                  `json:"AppArmorProfile"`
	EffectiveCaps   []string                `json:"EffectiveCaps"`
	BoundingCaps    []string                `json:"BoundingCaps"`
	ExecIDs         []string                `json:"ExecIDs"`
	GraphDriver     *driver.Data            `json:"GraphDriver"`
	SizeRw          int64                   `json:"SizeRw,omitempty"`
	SizeRootFs      int64                   `json:"SizeRootFs,omitempty"`
	Mounts          []specs.Mount           `json:"Mounts"`
	Dependencies    []string                `json:"Dependencies"`
	NetworkSettings *InspectNetworkSettings `json:"NetworkSettings"` //TODO
	ExitCommand     []string                `json:"ExitCommand"`
	Namespace       string                  `json:"Namespace"`
	IsInfra         bool                    `json:"IsInfra"`
}

// InspectContainerState provides a detailed record of a container's current
// state. It is returned as part of InspectContainerData.
// As with InspectContainerData, many portions of this struct are matched to
// Docker, but here we see more fields that are unused (nonsensical in the
// context of Libpod).
type InspectContainerState struct {
	OciVersion  string             `json:"OciVersion"`
	Status      string             `json:"Status"`
	Running     bool               `json:"Running"`
	Paused      bool               `json:"Paused"`
	Restarting  bool               `json:"Restarting"` // TODO
	OOMKilled   bool               `json:"OOMKilled"`
	Dead        bool               `json:"Dead"`
	Pid         int                `json:"Pid"`
	ExitCode    int32              `json:"ExitCode"`
	Error       string             `json:"Error"` // TODO
	StartedAt   time.Time          `json:"StartedAt"`
	FinishedAt  time.Time          `json:"FinishedAt"`
	Healthcheck HealthCheckResults `json:"Healthcheck,omitempty"`
}

// InspectNetworkSettings holds information about the network settings of the
// container.
// Many fields are maintained only for compatibility with `docker inspect` and
// are unused within Libpod.
type InspectNetworkSettings struct {
	Bridge                 string               `json:"Bridge"`
	SandboxID              string               `json:"SandboxID"`
	HairpinMode            bool                 `json:"HairpinMode"`
	LinkLocalIPv6Address   string               `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int                  `json:"LinkLocalIPv6PrefixLen"`
	Ports                  []ocicni.PortMapping `json:"Ports"`
	SandboxKey             string               `json:"SandboxKey"`
	SecondaryIPAddresses   []string             `json:"SecondaryIPAddresses"`
	SecondaryIPv6Addresses []string             `json:"SecondaryIPv6Addresses"`
	EndpointID             string               `json:"EndpointID"`
	Gateway                string               `json:"Gateway"`
	GlobalIPv6Address      string               `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen    int                  `json:"GlobalIPv6PrefixLen"`
	IPAddress              string               `json:"IPAddress"`
	IPPrefixLen            int                  `json:"IPPrefixLen"`
	IPv6Gateway            string               `json:"IPv6Gateway"`
	MacAddress             string               `json:"MacAddress"`
}

// Inspect a container for low-level information
func (c *Container) Inspect(size bool) (*InspectContainerData, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	storeCtr, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return nil, errors.Wrapf(err, "error getting container from store %q", c.ID())
	}
	layer, err := c.runtime.store.Layer(storeCtr.LayerID)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading information about layer %q", storeCtr.LayerID)
	}
	driverData, err := driver.GetDriverData(c.runtime.store, layer.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting graph driver info %q", c.ID())
	}
	return c.getContainerInspectData(size, driverData)
}

func (c *Container) getContainerInspectData(size bool, driverData *driver.Data) (*InspectContainerData, error) {
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

	data := &InspectContainerData{
		ID:      config.ID,
		Created: config.CreatedTime,
		Path:    path,
		Args:    args,
		State: &InspectContainerState{
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
		RestartCount:    int32(runtimeInfo.RestartCount),
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
		NetworkSettings: &InspectNetworkSettings{
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

	if c.state.ConfigPath != "" {
		data.OCIConfigPath = c.state.ConfigPath
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
