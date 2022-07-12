package libpod

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/driver"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/types"
	units "github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
)

// inspectLocked inspects a container for low-level information.
// The caller must held c.lock.
func (c *Container) inspectLocked(size bool) (*define.InspectContainerData, error) {
	storeCtr, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return nil, fmt.Errorf("error getting container from store %q: %w", c.ID(), err)
	}
	layer, err := c.runtime.store.Layer(storeCtr.LayerID)
	if err != nil {
		return nil, fmt.Errorf("error reading information about layer %q: %w", storeCtr.LayerID, err)
	}
	driverData, err := driver.GetDriverData(c.runtime.store, layer.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting graph driver info %q: %w", c.ID(), err)
	}
	return c.getContainerInspectData(size, driverData)
}

// Inspect a container for low-level information
func (c *Container) Inspect(size bool) (*define.InspectContainerData, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	return c.inspectLocked(size)
}

func (c *Container) volumesFrom() ([]string, error) {
	ctrSpec, err := c.specFromState()
	if err != nil {
		return nil, err
	}
	if ctrs, ok := ctrSpec.Annotations[define.InspectAnnotationVolumesFrom]; ok {
		return strings.Split(ctrs, ","), nil
	}
	return nil, nil
}

func (c *Container) getContainerInspectData(size bool, driverData *define.DriverData) (*define.InspectContainerData, error) {
	config := c.config
	runtimeInfo := c.state
	ctrSpec, err := c.specFromState()
	if err != nil {
		return nil, err
	}

	// Process is allowed to be nil in the stateSpec
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

	namedVolumes, mounts := c.SortUserVolumes(ctrSpec)
	inspectMounts, err := c.GetMounts(namedVolumes, c.config.ImageVolumes, mounts)
	if err != nil {
		return nil, err
	}

	cgroupPath, err := c.cGroupPath()
	if err != nil {
		// Handle the case where the container is not running or has no cgroup.
		if errors.Is(err, define.ErrNoCgroups) || errors.Is(err, define.ErrCtrStopped) {
			cgroupPath = ""
		} else {
			return nil, err
		}
	}

	data := &define.InspectContainerData{
		ID:      config.ID,
		Created: config.CreatedTime,
		Path:    path,
		Args:    args,
		State: &define.InspectContainerState{
			OciVersion:     ctrSpec.Version,
			Status:         runtimeInfo.State.String(),
			Running:        runtimeInfo.State == define.ContainerStateRunning,
			Paused:         runtimeInfo.State == define.ContainerStatePaused,
			OOMKilled:      runtimeInfo.OOMKilled,
			Dead:           runtimeInfo.State.String() == "bad state",
			Pid:            runtimeInfo.PID,
			ConmonPid:      runtimeInfo.ConmonPID,
			ExitCode:       runtimeInfo.ExitCode,
			Error:          "", // can't get yet
			StartedAt:      runtimeInfo.StartedTime,
			FinishedAt:     runtimeInfo.FinishedTime,
			Checkpointed:   runtimeInfo.Checkpointed,
			CgroupPath:     cgroupPath,
			RestoredAt:     runtimeInfo.RestoredTime,
			CheckpointedAt: runtimeInfo.CheckpointedTime,
			Restored:       runtimeInfo.Restored,
			CheckpointPath: runtimeInfo.CheckpointPath,
			CheckpointLog:  runtimeInfo.CheckpointLog,
			RestoreLog:     runtimeInfo.RestoreLog,
		},
		Image:           config.RootfsImageID,
		ImageName:       config.RootfsImageName,
		Namespace:       config.Namespace,
		Rootfs:          config.Rootfs,
		Pod:             config.Pod,
		ResolvConfPath:  resolvPath,
		HostnamePath:    hostnamePath,
		HostsPath:       hostsPath,
		StaticDir:       config.StaticDir,
		OCIRuntime:      config.OCIRuntime,
		ConmonPidFile:   config.ConmonPidFile,
		PidFile:         config.PidFile,
		Name:            config.Name,
		RestartCount:    int32(runtimeInfo.RestartCount),
		Driver:          driverData.Name,
		MountLabel:      config.MountLabel,
		ProcessLabel:    config.ProcessLabel,
		EffectiveCaps:   ctrSpec.Process.Capabilities.Effective,
		BoundingCaps:    ctrSpec.Process.Capabilities.Bounding,
		AppArmorProfile: ctrSpec.Process.ApparmorProfile,
		ExecIDs:         execIDs,
		GraphDriver:     driverData,
		Mounts:          inspectMounts,
		Dependencies:    c.Dependencies(),
		IsInfra:         c.IsInfra(),
		IsService:       c.IsService(),
	}

	if c.state.ConfigPath != "" {
		data.OCIConfigPath = c.state.ConfigPath
	}

	if c.config.HealthCheckConfig != nil {
		// This container has a healthcheck defined in it; we need to add it's state
		healthCheckState, err := c.getHealthCheckLog()
		if err != nil {
			// An error here is not considered fatal; no health state will be displayed
			logrus.Error(err)
		} else {
			data.State.Health = healthCheckState
		}
	}

	networkConfig, err := c.getContainerNetworkInfo()
	if err != nil {
		return nil, err
	}
	data.NetworkSettings = networkConfig

	inspectConfig := c.generateInspectContainerConfig(ctrSpec)
	data.Config = inspectConfig

	hostConfig, err := c.generateInspectContainerHostConfig(ctrSpec, namedVolumes, mounts)
	if err != nil {
		return nil, err
	}
	data.HostConfig = hostConfig

	if size {
		rootFsSize, err := c.rootFsSize()
		if err != nil {
			logrus.Errorf("Getting rootfs size %q: %v", config.ID, err)
		}
		data.SizeRootFs = rootFsSize

		rwSize, err := c.rwSize()
		if err != nil {
			logrus.Errorf("Getting rw size %q: %v", config.ID, err)
		}
		data.SizeRw = &rwSize
	}
	return data, nil
}

// Get inspect-formatted mounts list.
// Only includes user-specified mounts. Only includes bind mounts and named
// volumes, not tmpfs volumes.
func (c *Container) GetMounts(namedVolumes []*ContainerNamedVolume, imageVolumes []*ContainerImageVolume, mounts []spec.Mount) ([]define.InspectMount, error) {
	inspectMounts := []define.InspectMount{}

	// No mounts, return early
	if len(c.config.UserVolumes) == 0 {
		return inspectMounts, nil
	}

	for _, volume := range namedVolumes {
		mountStruct := define.InspectMount{}
		mountStruct.Type = "volume"
		mountStruct.Destination = volume.Dest
		mountStruct.Name = volume.Name

		// For src and driver, we need to look up the named
		// volume.
		volFromDB, err := c.runtime.state.Volume(volume.Name)
		if err != nil {
			return nil, fmt.Errorf("error looking up volume %s in container %s config: %w", volume.Name, c.ID(), err)
		}
		mountStruct.Driver = volFromDB.Driver()

		mountPoint, err := volFromDB.MountPoint()
		if err != nil {
			return nil, err
		}
		mountStruct.Source = mountPoint

		parseMountOptionsForInspect(volume.Options, &mountStruct)

		inspectMounts = append(inspectMounts, mountStruct)
	}

	for _, volume := range imageVolumes {
		mountStruct := define.InspectMount{}
		mountStruct.Type = "image"
		mountStruct.Destination = volume.Dest
		mountStruct.Source = volume.Source
		mountStruct.RW = volume.ReadWrite

		inspectMounts = append(inspectMounts, mountStruct)
	}

	for _, mount := range mounts {
		// It's a mount.
		// Is it a tmpfs? If so, discard.
		if mount.Type == "tmpfs" {
			continue
		}

		mountStruct := define.InspectMount{}
		mountStruct.Type = "bind"
		mountStruct.Source = mount.Source
		mountStruct.Destination = mount.Destination

		parseMountOptionsForInspect(mount.Options, &mountStruct)

		inspectMounts = append(inspectMounts, mountStruct)
	}

	return inspectMounts, nil
}

// GetSecurityOptions retrieves and returns the security related annotations and process information upon inspection
func (c *Container) GetSecurityOptions() []string {
	ctrSpec := c.config.Spec
	SecurityOpt := []string{}
	if ctrSpec.Process != nil {
		if ctrSpec.Process.NoNewPrivileges {
			SecurityOpt = append(SecurityOpt, "no-new-privileges")
		}
	}
	if label, ok := ctrSpec.Annotations[define.InspectAnnotationLabel]; ok {
		SecurityOpt = append(SecurityOpt, fmt.Sprintf("label=%s", label))
	}
	if seccomp, ok := ctrSpec.Annotations[define.InspectAnnotationSeccomp]; ok {
		SecurityOpt = append(SecurityOpt, fmt.Sprintf("seccomp=%s", seccomp))
	}
	if apparmor, ok := ctrSpec.Annotations[define.InspectAnnotationApparmor]; ok {
		SecurityOpt = append(SecurityOpt, fmt.Sprintf("apparmor=%s", apparmor))
	}
	return SecurityOpt
}

// Parse mount options so we can populate them in the mount structure.
// The mount passed in will be modified.
func parseMountOptionsForInspect(options []string, mount *define.InspectMount) {
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
		case "shared", "slave", "private", "rshared", "rslave", "rprivate", "unbindable", "runbindable":
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

// Generate the InspectContainerConfig struct for the Config field of Inspect.
func (c *Container) generateInspectContainerConfig(spec *spec.Spec) *define.InspectContainerConfig {
	ctrConfig := new(define.InspectContainerConfig)

	ctrConfig.Hostname = c.Hostname()
	ctrConfig.User = c.config.User
	if spec.Process != nil {
		ctrConfig.Tty = spec.Process.Terminal
		ctrConfig.Env = append([]string{}, spec.Process.Env...)
		ctrConfig.WorkingDir = spec.Process.Cwd
	}

	ctrConfig.StopTimeout = c.config.StopTimeout
	ctrConfig.Timeout = c.config.Timeout
	ctrConfig.OpenStdin = c.config.Stdin
	ctrConfig.Image = c.config.RootfsImageName
	ctrConfig.SystemdMode = c.Systemd()

	// Leave empty is not explicitly overwritten by user
	if len(c.config.Command) != 0 {
		ctrConfig.Cmd = []string{}
		ctrConfig.Cmd = append(ctrConfig.Cmd, c.config.Command...)
	}

	// Leave empty if not explicitly overwritten by user
	if len(c.config.Entrypoint) != 0 {
		ctrConfig.Entrypoint = strings.Join(c.config.Entrypoint, " ")
	}

	if len(c.config.Labels) != 0 {
		ctrConfig.Labels = make(map[string]string)
		for k, v := range c.config.Labels {
			ctrConfig.Labels[k] = v
		}
	}

	if len(spec.Annotations) != 0 {
		ctrConfig.Annotations = make(map[string]string)
		for k, v := range spec.Annotations {
			ctrConfig.Annotations[k] = v
		}
	}

	ctrConfig.StopSignal = c.config.StopSignal
	// TODO: should JSON deep copy this to ensure internal pointers don't
	// leak.
	ctrConfig.Healthcheck = c.config.HealthCheckConfig

	ctrConfig.CreateCommand = c.config.CreateCommand

	ctrConfig.Timezone = c.config.Timezone
	for _, secret := range c.config.Secrets {
		newSec := define.InspectSecret{}
		newSec.Name = secret.Name
		newSec.ID = secret.ID
		newSec.UID = secret.UID
		newSec.GID = secret.GID
		newSec.Mode = secret.Mode
		ctrConfig.Secrets = append(ctrConfig.Secrets, &newSec)
	}

	// Pad Umask to 4 characters
	if len(c.config.Umask) < 4 {
		pad := strings.Repeat("0", 4-len(c.config.Umask))
		ctrConfig.Umask = pad + c.config.Umask
	} else {
		ctrConfig.Umask = c.config.Umask
	}

	ctrConfig.Passwd = c.config.Passwd
	ctrConfig.ChrootDirs = append(ctrConfig.ChrootDirs, c.config.ChrootDirs...)

	return ctrConfig
}

func generateIDMappings(idMappings types.IDMappingOptions) *define.InspectIDMappings {
	var inspectMappings define.InspectIDMappings
	for _, uid := range idMappings.UIDMap {
		inspectMappings.UIDMap = append(inspectMappings.UIDMap, fmt.Sprintf("%d:%d:%d", uid.ContainerID, uid.HostID, uid.Size))
	}
	for _, gid := range idMappings.GIDMap {
		inspectMappings.GIDMap = append(inspectMappings.GIDMap, fmt.Sprintf("%d:%d:%d", gid.ContainerID, gid.HostID, gid.Size))
	}
	return &inspectMappings
}

// Generate the InspectContainerHostConfig struct for the HostConfig field of
// Inspect.
func (c *Container) generateInspectContainerHostConfig(ctrSpec *spec.Spec, namedVolumes []*ContainerNamedVolume, mounts []spec.Mount) (*define.InspectContainerHostConfig, error) {
	hostConfig := new(define.InspectContainerHostConfig)

	logConfig := new(define.InspectLogConfig)
	logConfig.Type = c.config.LogDriver
	logConfig.Path = c.config.LogPath
	logConfig.Size = units.HumanSize(float64(c.config.LogSize))
	logConfig.Tag = c.config.LogTag

	hostConfig.LogConfig = logConfig

	restartPolicy := new(define.InspectRestartPolicy)
	restartPolicy.Name = c.config.RestartPolicy
	restartPolicy.MaximumRetryCount = c.config.RestartRetries
	hostConfig.RestartPolicy = restartPolicy
	if c.config.NoCgroups {
		hostConfig.Cgroups = "disabled"
	} else {
		hostConfig.Cgroups = "default"
	}

	hostConfig.Dns = make([]string, 0, len(c.config.DNSServer))
	for _, dns := range c.config.DNSServer {
		hostConfig.Dns = append(hostConfig.Dns, dns.String())
	}

	hostConfig.DnsOptions = make([]string, 0, len(c.config.DNSOption))
	hostConfig.DnsOptions = append(hostConfig.DnsOptions, c.config.DNSOption...)

	hostConfig.DnsSearch = make([]string, 0, len(c.config.DNSSearch))
	hostConfig.DnsSearch = append(hostConfig.DnsSearch, c.config.DNSSearch...)

	hostConfig.ExtraHosts = make([]string, 0, len(c.config.HostAdd))
	hostConfig.ExtraHosts = append(hostConfig.ExtraHosts, c.config.HostAdd...)

	hostConfig.GroupAdd = make([]string, 0, len(c.config.Groups))
	hostConfig.GroupAdd = append(hostConfig.GroupAdd, c.config.Groups...)

	if ctrSpec.Process != nil {
		if ctrSpec.Process.OOMScoreAdj != nil {
			hostConfig.OomScoreAdj = *ctrSpec.Process.OOMScoreAdj
		}
	}

	hostConfig.SecurityOpt = c.GetSecurityOptions()

	hostConfig.ReadonlyRootfs = ctrSpec.Root.Readonly
	hostConfig.ShmSize = c.config.ShmSize
	hostConfig.Runtime = "oci"

	// This is very expensive to initialize.
	// So we don't want to initialize it unless we absolutely have to - IE,
	// there are things that require a major:minor to path translation.
	var deviceNodes map[string]string

	// Annotations
	if ctrSpec.Annotations != nil {
		hostConfig.ContainerIDFile = ctrSpec.Annotations[define.InspectAnnotationCIDFile]
		if ctrSpec.Annotations[define.InspectAnnotationAutoremove] == define.InspectResponseTrue {
			hostConfig.AutoRemove = true
		}
		if ctrs, ok := ctrSpec.Annotations[define.InspectAnnotationVolumesFrom]; ok {
			hostConfig.VolumesFrom = strings.Split(ctrs, ",")
		}
		if ctrSpec.Annotations[define.InspectAnnotationPrivileged] == define.InspectResponseTrue {
			hostConfig.Privileged = true
		}
		if ctrSpec.Annotations[define.InspectAnnotationInit] == define.InspectResponseTrue {
			hostConfig.Init = true
		}
	}

	// Resource limits
	if ctrSpec.Linux != nil {
		if ctrSpec.Linux.Resources != nil {
			if ctrSpec.Linux.Resources.CPU != nil {
				if ctrSpec.Linux.Resources.CPU.Shares != nil {
					hostConfig.CpuShares = *ctrSpec.Linux.Resources.CPU.Shares
				}
				if ctrSpec.Linux.Resources.CPU.Period != nil {
					hostConfig.CpuPeriod = *ctrSpec.Linux.Resources.CPU.Period
				}
				if ctrSpec.Linux.Resources.CPU.Quota != nil {
					hostConfig.CpuQuota = *ctrSpec.Linux.Resources.CPU.Quota
				}
				if ctrSpec.Linux.Resources.CPU.RealtimePeriod != nil {
					hostConfig.CpuRealtimePeriod = *ctrSpec.Linux.Resources.CPU.RealtimePeriod
				}
				if ctrSpec.Linux.Resources.CPU.RealtimeRuntime != nil {
					hostConfig.CpuRealtimeRuntime = *ctrSpec.Linux.Resources.CPU.RealtimeRuntime
				}
				hostConfig.CpusetCpus = ctrSpec.Linux.Resources.CPU.Cpus
				hostConfig.CpusetMems = ctrSpec.Linux.Resources.CPU.Mems
			}
			if ctrSpec.Linux.Resources.Memory != nil {
				if ctrSpec.Linux.Resources.Memory.Limit != nil {
					hostConfig.Memory = *ctrSpec.Linux.Resources.Memory.Limit
				}
				if ctrSpec.Linux.Resources.Memory.Reservation != nil {
					hostConfig.MemoryReservation = *ctrSpec.Linux.Resources.Memory.Reservation
				}
				if ctrSpec.Linux.Resources.Memory.Swap != nil {
					hostConfig.MemorySwap = *ctrSpec.Linux.Resources.Memory.Swap
				}
				if ctrSpec.Linux.Resources.Memory.Swappiness != nil {
					hostConfig.MemorySwappiness = int64(*ctrSpec.Linux.Resources.Memory.Swappiness)
				} else {
					// Swappiness has a default of -1
					hostConfig.MemorySwappiness = -1
				}
				if ctrSpec.Linux.Resources.Memory.DisableOOMKiller != nil {
					hostConfig.OomKillDisable = *ctrSpec.Linux.Resources.Memory.DisableOOMKiller
				}
			}
			if ctrSpec.Linux.Resources.Pids != nil {
				hostConfig.PidsLimit = ctrSpec.Linux.Resources.Pids.Limit
			}
			hostConfig.CgroupConf = ctrSpec.Linux.Resources.Unified
			if ctrSpec.Linux.Resources.BlockIO != nil {
				if ctrSpec.Linux.Resources.BlockIO.Weight != nil {
					hostConfig.BlkioWeight = *ctrSpec.Linux.Resources.BlockIO.Weight
				}
				hostConfig.BlkioWeightDevice = []define.InspectBlkioWeightDevice{}
				for _, dev := range ctrSpec.Linux.Resources.BlockIO.WeightDevice {
					key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
					// TODO: how do we handle LeafWeight vs
					// Weight? For now, ignore anything
					// without Weight set.
					if dev.Weight == nil {
						logrus.Infof("Ignoring weight device %s as it lacks a weight", key)
						continue
					}
					if deviceNodes == nil {
						nodes, err := util.FindDeviceNodes()
						if err != nil {
							return nil, err
						}
						deviceNodes = nodes
					}
					path, ok := deviceNodes[key]
					if !ok {
						logrus.Infof("Could not locate weight device %s in system devices", key)
						continue
					}
					weightDev := define.InspectBlkioWeightDevice{}
					weightDev.Path = path
					weightDev.Weight = *dev.Weight
					hostConfig.BlkioWeightDevice = append(hostConfig.BlkioWeightDevice, weightDev)
				}

				readBps, err := blkioDeviceThrottle(deviceNodes, ctrSpec.Linux.Resources.BlockIO.ThrottleReadBpsDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceReadBps = readBps

				writeBps, err := blkioDeviceThrottle(deviceNodes, ctrSpec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceWriteBps = writeBps

				readIops, err := blkioDeviceThrottle(deviceNodes, ctrSpec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceReadIOps = readIops

				writeIops, err := blkioDeviceThrottle(deviceNodes, ctrSpec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceWriteIOps = writeIops
			}
		}
	}

	// NanoCPUs.
	// This is only calculated if CpuPeriod == 100000.
	// It is given in nanoseconds, versus the microseconds used elsewhere -
	// so multiply by 10000 (not sure why, but 1000 is off by 10).
	if hostConfig.CpuPeriod == 100000 {
		hostConfig.NanoCpus = 10000 * hostConfig.CpuQuota
	}

	// Bind mounts, formatted as src:dst.
	// We'll be appending some options that aren't necessarily in the
	// original command line... but no helping that from inside libpod.
	binds := []string{}
	tmpfs := make(map[string]string)
	for _, namedVol := range namedVolumes {
		if len(namedVol.Options) > 0 {
			binds = append(binds, fmt.Sprintf("%s:%s:%s", namedVol.Name, namedVol.Dest, strings.Join(namedVol.Options, ",")))
		} else {
			binds = append(binds, fmt.Sprintf("%s:%s", namedVol.Name, namedVol.Dest))
		}
	}
	for _, mount := range mounts {
		if mount.Type == "tmpfs" {
			tmpfs[mount.Destination] = strings.Join(mount.Options, ",")
		} else {
			// TODO - maybe we should parse for empty source/destination
			// here. Would be confusing if we print just a bare colon.
			if len(mount.Options) > 0 {
				binds = append(binds, fmt.Sprintf("%s:%s:%s", mount.Source, mount.Destination, strings.Join(mount.Options, ",")))
			} else {
				binds = append(binds, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
			}
		}
	}
	hostConfig.Binds = binds
	hostConfig.Tmpfs = tmpfs

	// Network mode parsing.
	networkMode := c.NetworkMode()
	hostConfig.NetworkMode = networkMode

	// Port bindings.
	// Only populate if we're using CNI to configure the network.
	if c.config.CreateNetNS {
		hostConfig.PortBindings = makeInspectPortBindings(c.config.PortMappings, c.config.ExposedPorts)
	} else {
		hostConfig.PortBindings = make(map[string][]define.InspectHostPort)
	}

	// Cap add and cap drop.
	// We need a default set of capabilities to compare against.
	// The OCI generate package has one, and is commonly used, so we'll
	// use it.
	// Problem: there are 5 sets of capabilities.
	// Use the bounding set for this computation, it's the most encompassing
	// (but still not perfect).
	capAdd := []string{}
	capDrop := []string{}
	// No point in continuing if we got a spec without a Process block...
	if ctrSpec.Process != nil {
		// Max an O(1) lookup table for default bounding caps.
		boundingCaps := make(map[string]bool)
		g, err := generate.New("linux")
		if err != nil {
			return nil, err
		}
		if !hostConfig.Privileged {
			for _, cap := range g.Config.Process.Capabilities.Bounding {
				boundingCaps[cap] = true
			}
		} else {
			// If we are privileged, use all caps.
			for _, cap := range capability.List() {
				if g.HostSpecific && cap > validate.LastCap() {
					continue
				}
				boundingCaps[fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String()))] = true
			}
		}
		// Iterate through spec caps.
		// If it's not in default bounding caps, it was added.
		// If it is, delete from the default set. Whatever remains after
		// we finish are the dropped caps.
		for _, cap := range ctrSpec.Process.Capabilities.Bounding {
			if _, ok := boundingCaps[cap]; ok {
				delete(boundingCaps, cap)
			} else {
				capAdd = append(capAdd, cap)
			}
		}
		for cap := range boundingCaps {
			capDrop = append(capDrop, cap)
		}
		// Sort CapDrop so it displays in consistent order (GH #9490)
		sort.Strings(capDrop)
	}
	hostConfig.CapAdd = capAdd
	hostConfig.CapDrop = capDrop
	switch {
	case c.config.IPCNsCtr != "":
		hostConfig.IpcMode = fmt.Sprintf("container:%s", c.config.IPCNsCtr)
	case ctrSpec.Linux != nil:
		// Locate the spec's IPC namespace.
		// If there is none, it's ipc=host.
		// If there is one and it has a path, it's "ns:".
		// If no path, it's default - the empty string.
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.IPCNamespace {
				if ns.Path != "" {
					hostConfig.IpcMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					break
				}
			}
		}
	case c.config.NoShm:
		hostConfig.IpcMode = "none"
	case c.config.NoShmShare:
		hostConfig.IpcMode = "private"
	}
	if hostConfig.IpcMode == "" {
		hostConfig.IpcMode = "shareable"
	}

	// Cgroup namespace mode
	cgroupMode := ""
	if c.config.CgroupNsCtr != "" {
		cgroupMode = fmt.Sprintf("container:%s", c.config.CgroupNsCtr)
	} else if ctrSpec.Linux != nil {
		// Locate the spec's cgroup namespace
		// If there is none, it's cgroup=host.
		// If there is one and it has a path, it's "ns:".
		// If there is no path, it's private.
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.CgroupNamespace {
				if ns.Path != "" {
					cgroupMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					cgroupMode = "private"
				}
			}
		}
		if cgroupMode == "" {
			cgroupMode = "host"
		}
	}
	hostConfig.CgroupMode = cgroupMode

	// Cgroup parent
	// Need to check if it's the default, and not print if so.
	defaultCgroupParent := ""
	switch c.CgroupManager() {
	case config.CgroupfsCgroupsManager:
		defaultCgroupParent = CgroupfsDefaultCgroupParent
	case config.SystemdCgroupsManager:
		defaultCgroupParent = SystemdDefaultCgroupParent
	}
	if c.config.CgroupParent != defaultCgroupParent {
		hostConfig.CgroupParent = c.config.CgroupParent
	}
	hostConfig.CgroupManager = c.CgroupManager()

	// PID namespace mode
	pidMode := ""
	if c.config.PIDNsCtr != "" {
		pidMode = fmt.Sprintf("container:%s", c.config.PIDNsCtr)
	} else if ctrSpec.Linux != nil {
		// Locate the spec's PID namespace.
		// If there is none, it's pid=host.
		// If there is one and it has a path, it's "ns:".
		// If there is no path, it's default - the empty string.
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.PIDNamespace {
				if ns.Path != "" {
					pidMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					pidMode = "private"
				}
				break
			}
		}
		if pidMode == "" {
			pidMode = "host"
		}
	}
	hostConfig.PidMode = pidMode

	// UTS namespace mode
	utsMode := c.NamespaceMode(spec.UTSNamespace, ctrSpec)

	hostConfig.UTSMode = utsMode

	// User namespace mode
	usernsMode := ""
	if c.config.UserNsCtr != "" {
		usernsMode = fmt.Sprintf("container:%s", c.config.UserNsCtr)
	} else if ctrSpec.Linux != nil {
		// Locate the spec's user namespace.
		// If there is none, it's default - the empty string.
		// If there is one, it's "private" if no path, or "ns:" if
		// there's a path.

		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.UserNamespace {
				if ns.Path != "" {
					usernsMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					usernsMode = "private"
				}
			}
		}
	}
	hostConfig.UsernsMode = usernsMode
	if c.config.IDMappings.UIDMap != nil && c.config.IDMappings.GIDMap != nil {
		hostConfig.IDMappings = generateIDMappings(c.config.IDMappings)
	}
	// Devices
	// Do not include if privileged - assumed that all devices will be
	// included.
	var err error
	hostConfig.Devices, err = c.GetDevices(hostConfig.Privileged, *ctrSpec, deviceNodes)
	if err != nil {
		return nil, err
	}

	// Ulimits
	hostConfig.Ulimits = []define.InspectUlimit{}
	if ctrSpec.Process != nil {
		for _, limit := range ctrSpec.Process.Rlimits {
			newLimit := define.InspectUlimit{}
			newLimit.Name = limit.Type
			newLimit.Soft = int64(limit.Soft)
			newLimit.Hard = int64(limit.Hard)
			hostConfig.Ulimits = append(hostConfig.Ulimits, newLimit)
		}
	}

	// Terminal size
	// We can't actually get this for now...
	// So default to something sane.
	// TODO: Populate this.
	hostConfig.ConsoleSize = []uint{0, 0}

	return hostConfig, nil
}

// Return true if the container is running in the host's PID NS.
func (c *Container) inHostPidNS() (bool, error) {
	if c.config.PIDNsCtr != "" {
		return false, nil
	}
	ctrSpec, err := c.specFromState()
	if err != nil {
		return false, err
	}
	if ctrSpec.Linux != nil {
		// Locate the spec's PID namespace.
		// If there is none, it's pid=host.
		// If there is one and it has a path, it's "ns:".
		// If there is no path, it's default - the empty string.
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.PIDNamespace {
				return false, nil
			}
		}
	}
	return true, nil
}

func (c *Container) GetDevices(priv bool, ctrSpec spec.Spec, deviceNodes map[string]string) ([]define.InspectDevice, error) {
	devices := []define.InspectDevice{}
	if ctrSpec.Linux != nil && !priv {
		for _, dev := range ctrSpec.Linux.Devices {
			key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
			if deviceNodes == nil {
				nodes, err := util.FindDeviceNodes()
				if err != nil {
					return nil, err
				}
				deviceNodes = nodes
			}
			path, ok := deviceNodes[key]
			if !ok {
				logrus.Warnf("Could not locate device %s on host", key)
				continue
			}
			newDev := define.InspectDevice{}
			newDev.PathOnHost = path
			newDev.PathInContainer = dev.Path
			devices = append(devices, newDev)
		}
	}
	return devices, nil
}

func blkioDeviceThrottle(deviceNodes map[string]string, devs []spec.LinuxThrottleDevice) ([]define.InspectBlkioThrottleDevice, error) {
	out := []define.InspectBlkioThrottleDevice{}
	for _, dev := range devs {
		key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
		if deviceNodes == nil {
			nodes, err := util.FindDeviceNodes()
			if err != nil {
				return nil, err
			}
			deviceNodes = nodes
		}
		path, ok := deviceNodes[key]
		if !ok {
			logrus.Infof("Could not locate throttle device %s in system devices", key)
			continue
		}
		throttleDev := define.InspectBlkioThrottleDevice{}
		throttleDev.Path = path
		throttleDev.Rate = dev.Rate
		out = append(out, throttleDev)
	}
	return out, nil
}
