package generate

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func setProcOpts(s *specgen.SpecGenerator, g *generate.Generator) {
	if s.ProcOpts == nil {
		return
	}
	for i := range g.Config.Mounts {
		if g.Config.Mounts[i].Destination == "/proc" {
			g.Config.Mounts[i].Options = s.ProcOpts
			return
		}
	}
}

func addRlimits(s *specgen.SpecGenerator, g *generate.Generator) {
	var (
		isRootless = rootless.IsRootless()
		nofileSet  = false
		nprocSet   = false
	)

	if s.Rlimits == nil {
		g.Config.Process.Rlimits = nil
		return
	}

	for _, u := range s.Rlimits {
		name := "RLIMIT_" + strings.ToUpper(u.Type)
		if name == "RLIMIT_NOFILE" {
			nofileSet = true
		} else if name == "RLIMIT_NPROC" {
			nprocSet = true
		}
		g.AddProcessRlimits(name, u.Hard, u.Soft)
	}

	// If not explicitly overridden by the user, default number of open
	// files and number of processes to the maximum they can be set to
	// (without overriding a sysctl)
	if !nofileSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NOFILE ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NOFILE", max, current)
	}
	if !nprocSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NPROC ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NPROC", max, current)
	}
}

// Produce the final command for the container.
func makeCommand(s *specgen.SpecGenerator, imageData *libimage.ImageData, rtc *config.Config) ([]string, error) {
	finalCommand := []string{}

	entrypoint := s.Entrypoint
	if entrypoint == nil && imageData != nil {
		entrypoint = imageData.Config.Entrypoint
	}

	// Don't append the entrypoint if it is [""]
	if len(entrypoint) != 1 || entrypoint[0] != "" {
		finalCommand = append(finalCommand, entrypoint...)
	}

	// Only use image command if the user did not manually set an
	// entrypoint.
	command := s.Command
	if len(command) == 0 && imageData != nil && len(s.Entrypoint) == 0 {
		command = imageData.Config.Cmd
	}

	finalCommand = append(finalCommand, command...)

	if len(finalCommand) == 0 {
		return nil, errors.Errorf("no command or entrypoint provided, and no CMD or ENTRYPOINT from image")
	}

	if s.Init {
		initPath := s.InitPath
		if initPath == "" && rtc != nil {
			initPath = rtc.Engine.InitPath
		}
		if initPath == "" {
			return nil, errors.Errorf("no path to init binary found but container requested an init")
		}
		finalCommand = append([]string{"/dev/init", "--"}, finalCommand...)
	}

	return finalCommand, nil
}

// canMountSys is a best-effort heuristic to detect whether mounting a new sysfs is permitted in the container
func canMountSys(isRootless, isNewUserns bool, s *specgen.SpecGenerator) bool {
	if s.NetNS.IsHost() && (isRootless || isNewUserns) {
		return false
	}
	if isNewUserns {
		switch s.NetNS.NSMode {
		case specgen.Slirp, specgen.Private, specgen.NoNetwork, specgen.Bridge:
			return true
		default:
			return false
		}
	}
	return true
}

func getCgroupPermissons(unmask []string) string {
	ro := "ro"
	rw := "rw"
	cgroup := "/sys/fs/cgroup"

	cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
	if !cgroupv2 {
		return ro
	}

	if unmask != nil && unmask[0] == "ALL" {
		return rw
	}

	for _, p := range unmask {
		if path.Clean(p) == cgroup {
			return rw
		}
	}
	return ro
}

// SpecGenToOCI returns the base configuration for the container.
func SpecGenToOCI(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *libimage.Image, mounts []spec.Mount, pod *libpod.Pod, finalCmd []string, compatibleOptions *libpod.InfraInherit) (*spec.Spec, error) {
	cgroupPerm := getCgroupPermissons(s.Unmask)

	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")
	g.HostSpecific = true
	addCgroup := true

	isRootless := rootless.IsRootless()
	isNewUserns := s.UserNS.IsContainer() || s.UserNS.IsPath() || s.UserNS.IsPrivate()

	canMountSys := canMountSys(isRootless, isNewUserns, s)

	if s.Privileged && canMountSys {
		cgroupPerm = "rw"
		g.RemoveMount("/sys")
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "rw"},
		}
		g.AddMount(sysMnt)
	}
	if !canMountSys {
		addCgroup = false
		g.RemoveMount("/sys")
		r := "ro"
		if s.Privileged {
			r = "rw"
		}
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "bind", // should we use a constant for this, like createconfig?
			Source:      "/sys",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", r, "rbind"},
		}
		g.AddMount(sysMnt)
		if !s.Privileged && isRootless {
			g.AddLinuxMaskedPaths("/sys/kernel")
		}
	}
	gid5Available := true
	if isRootless {
		nGids, err := rootless.GetAvailableGids()
		if err != nil {
			return nil, err
		}
		gid5Available = nGids >= 5
	}
	// When using a different user namespace, check that the GID 5 is mapped inside
	// the container.
	if gid5Available && (s.IDMappings != nil && len(s.IDMappings.GIDMap) > 0) {
		mappingFound := false
		for _, r := range s.IDMappings.GIDMap {
			if r.ContainerID <= 5 && 5 < r.ContainerID+r.Size {
				mappingFound = true
				break
			}
		}
		if !mappingFound {
			gid5Available = false
		}
	}
	if !gid5Available {
		// If we have no GID mappings, the gid=5 default option would fail, so drop it.
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"rprivate", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	inUserNS := isRootless || isNewUserns

	if inUserNS && s.IpcNS.IsHost() {
		g.RemoveMount("/dev/mqueue")
		devMqueue := spec.Mount{
			Destination: "/dev/mqueue",
			Type:        "bind", // constant ?
			Source:      "/dev/mqueue",
			Options:     []string{"bind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(devMqueue)
	}
	if inUserNS && s.PidNS.IsHost() {
		g.RemoveMount("/proc")
		procMount := spec.Mount{
			Destination: "/proc",
			Type:        define.TypeBind,
			Source:      "/proc",
			Options:     []string{"rbind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(procMount)
	}

	if addCgroup {
		cgroupMnt := spec.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "relatime", cgroupPerm},
		}
		g.AddMount(cgroupMnt)
	}

	g.Config.Linux.Personality = s.Personality

	g.SetProcessCwd(s.WorkDir)

	g.SetProcessArgs(finalCmd)

	g.SetProcessTerminal(s.Terminal)

	for key, val := range s.Annotations {
		g.AddAnnotation(key, val)
	}

	switch {
	case compatibleOptions.InfraResources == nil && s.ResourceLimits != nil:
		out, err := json.Marshal(s.ResourceLimits)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(out, g.Config.Linux.Resources)
		if err != nil {
			return nil, err
		}
	case s.ResourceLimits != nil: // if we have predefined resource limits we need to make sure we keep the infra and container limits
		originalResources, err := json.Marshal(s.ResourceLimits)
		if err != nil {
			return nil, err
		}
		infraResources, err := json.Marshal(compatibleOptions.InfraResources)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(infraResources, s.ResourceLimits) // put infra's resource limits in the container
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(originalResources, s.ResourceLimits) // make sure we did not override anything
		if err != nil {
			return nil, err
		}
		g.Config.Linux.Resources = s.ResourceLimits
	default:
		g.Config.Linux.Resources = compatibleOptions.InfraResources
	}
	// Devices

	// set the default rule at the beginning of device configuration
	if !inUserNS && !s.Privileged {
		g.AddLinuxResourcesDevice(false, "", nil, nil, "rwm")
	}

	var userDevices []spec.LinuxDevice
	if s.Privileged {
		// If privileged, we need to add all the host devices to the
		// spec.  We do not add the user provided ones because we are
		// already adding them all.
		if err := addPrivilegedDevices(&g); err != nil {
			return nil, err
		}
	} else {
		// add default devices from containers.conf
		for _, device := range rtc.Containers.Devices {
			if err = DevicesFromPath(&g, device); err != nil {
				return nil, err
			}
		}
		if len(compatibleOptions.HostDeviceList) > 0 && len(s.Devices) == 0 {
			userDevices = compatibleOptions.HostDeviceList
		} else {
			userDevices = s.Devices
		}
		// add default devices specified by caller
		for _, device := range userDevices {
			if err = DevicesFromPath(&g, device.Path); err != nil {
				return nil, err
			}
		}
	}
	s.HostDeviceList = userDevices

	// set the devices cgroup when not running in a user namespace
	if !inUserNS && !s.Privileged {
		for _, dev := range s.DeviceCgroupRule {
			g.AddLinuxResourcesDevice(true, dev.Type, dev.Major, dev.Minor, dev.Access)
		}
	}

	for k, v := range s.WeightDevice {
		statT := unix.Stat_t{}
		if err := unix.Stat(k, &statT); err != nil {
			return nil, errors.Wrapf(err, "failed to inspect '%s' in --blkio-weight-device", k)
		}
		g.AddLinuxResourcesBlockIOWeightDevice((int64(unix.Major(uint64(statT.Rdev)))), (int64(unix.Minor(uint64(statT.Rdev)))), *v.Weight) // nolint: unconvert
	}

	BlockAccessToKernelFilesystems(s.Privileged, s.PidNS.IsHost(), s.Mask, s.Unmask, &g)

	g.ClearProcessEnv()
	for name, val := range s.Env {
		g.AddProcessEnv(name, val)
	}

	addRlimits(s, &g)

	// NAMESPACES
	if err := specConfigureNamespaces(s, &g, rt, pod); err != nil {
		return nil, err
	}
	configSpec := g.Config

	if err := securityConfigureGenerator(s, &g, newImage, rtc); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = SupersedeUserMounts(mounts, configSpec.Mounts)
	// Process mounts to ensure correct options
	if err := InitFSMounts(configSpec.Mounts); err != nil {
		return nil, err
	}

	// Add annotations
	if configSpec.Annotations == nil {
		configSpec.Annotations = make(map[string]string)
	}

	if s.Remove {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseFalse
	}

	if len(s.VolumesFrom) > 0 {
		configSpec.Annotations[define.InspectAnnotationVolumesFrom] = strings.Join(s.VolumesFrom, ",")
	}

	if s.Privileged {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseFalse
	}

	if s.Init {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseFalse
	}

	if s.OOMScoreAdj != nil {
		g.SetProcessOOMScoreAdj(*s.OOMScoreAdj)
	}
	setProcOpts(s, &g)

	return configSpec, nil
}
