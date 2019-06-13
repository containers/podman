package libpod

import (
	"time"

	"github.com/containers/libpod/libpod/driver"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
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
	Mounts          []*InspectMount         `json:"Mounts"`
	Dependencies    []string                `json:"Dependencies"`
	NetworkSettings *InspectNetworkSettings `json:"NetworkSettings"` //TODO
	ExitCommand     []string                `json:"ExitCommand"`
	Namespace       string                  `json:"Namespace"`
	IsInfra         bool                    `json:"IsInfra"`
}

// InspectMount provides a record of a single mount in a container. It contains
// fields for both named and normal volumes. Only user-specified volumes will be
// included, and tmpfs volumes are not included even if the user specified them.
type InspectMount struct {
	// Whether the mount is a volume or bind mount. Allowed values are
	// "volume" and "bind".
	Type string `json:"Type"`
	// The name of the volume. Empty for bind mounts.
	Name string `json:"Name,omptempty"`
	// The source directory for the volume.
	Src string `json:"Source"`
	// The destination directory for the volume. Specified as a path within
	// the container, as it would be passed into the OCI runtime.
	Dst string `json:"Destination"`
	// The driver used for the named volume. Empty for bind mounts.
	Driver string `json:"Driver"`
	// Contains SELinux :z/:Z mount options. Unclear what, if anything, else
	// goes in here.
	Mode string `json:"Mode"`
	// All remaining mount options. Additional data, not present in the
	// original output.
	Options []string `json:"Options"`
	// Whether the volume is read-write
	RW bool `json:"RW"`
	// Mount propagation for the mount. Can be empty if not specified, but
	// is always printed - no omitempty.
	Propagation string `json:"Propagation"`
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

	resolvPath := ""
	hostsPath := ""
	hostnamePath := ""
	if c.state.BindMounts != nil {
		if getPath, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
			resolvPath = getPath
		}
		if getPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
			hostsPath = getPath
		}
		if getPath, ok := c.state.BindMounts["/etc/hostname"]; ok {
			hostnamePath = getPath
		}
	}

	mounts, err := c.getInspectMounts()
	if err != nil {
		return nil, err
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
		// This container has a healthcheck defined in it; we need to add it's state
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

// Get inspect-formatted mounts list.
// Only includes user-specified mounts. Only includes bind mounts and named
// volumes, not tmpfs volumes.
func (c *Container) getInspectMounts() ([]*InspectMount, error) {
	inspectMounts := []*InspectMount{}

	// No mounts, return early
	if len(c.config.UserVolumes) == 0 {
		return inspectMounts, nil
	}

	// We need to parse all named volumes and mounts into maps, so we don't
	// end up with repeated lookups for each user volume.
	// Map destination to struct, as destination is what is stored in
	// UserVolumes.
	namedVolumes := make(map[string]*ContainerNamedVolume)
	mounts := make(map[string]spec.Mount)
	for _, namedVol := range c.config.NamedVolumes {
		namedVolumes[namedVol.Dest] = namedVol
	}
	for _, mount := range c.config.Spec.Mounts {
		mounts[mount.Destination] = mount
	}

	for _, vol := range c.config.UserVolumes {
		// We need to look up the volumes.
		// First: is it a named volume?
		if volume, ok := namedVolumes[vol]; ok {
			mountStruct := new(InspectMount)
			mountStruct.Type = "volume"
			mountStruct.Dst = volume.Dest
			mountStruct.Name = volume.Name

			// For src and driver, we need to look up the named
			// volume.
			volFromDB, err := c.runtime.state.Volume(volume.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "error looking up volume %s in container %s config", volume.Name, c.ID())
			}
			mountStruct.Driver = volFromDB.Driver()
			mountStruct.Src = volFromDB.MountPoint()

			parseMountOptionsForInspect(volume.Options, mountStruct)

			inspectMounts = append(inspectMounts, mountStruct)
		} else if mount, ok := mounts[vol]; ok {
			// It's a mount.
			// Is it a tmpfs? If so, discard.
			if mount.Type == "tmpfs" {
				continue
			}

			mountStruct := new(InspectMount)
			mountStruct.Type = "bind"
			mountStruct.Src = mount.Source
			mountStruct.Dst = mount.Destination

			parseMountOptionsForInspect(mount.Options, mountStruct)

			inspectMounts = append(inspectMounts, mountStruct)
		}
		// We couldn't find a mount. Log a warning.
		logrus.Warnf("Could not find mount at destination %q when building inspect output for container %s", vol, c.ID())
	}

	return inspectMounts, nil
}

// Parse mount options so we can populate them in the mount structure.
// The mount passed in will be modified.
func parseMountOptionsForInspect(options []string, mount *InspectMount) {
	isRW := true
	mountProp := ""
	zZ := ""
	otherOpts := []string{}

	// Some of these may be overwritten if the user passes us garbage opts
	// (for example, [ro,rw])
	// We catch these on the Podman side, so not a problem there, but other
	// users of libpod who do not properly validate mount options may see
	// this.
	// Not really worth dealing with on our end - garbage in, garbage out.
	for _, opt := range options {
		switch opt {
		case "ro":
			isRW = false
		case "rw":
			// Do nothing, silently discard
		case "shared", "slave", "private", "rshared", "rslave", "rprivate":
			mountProp = opt
		case "z", "Z":
			zZ = opt
		default:
			otherOpts = append(otherOpts, opt)
		}
	}

	mount.RW = isRW
	mount.Propagation = mountProp
	mount.Mode = zZ
	mount.Options = otherOpts
}
