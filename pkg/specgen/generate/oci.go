package generate

import (
	"context"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func addRlimits(s *specgen.SpecGenerator, g *generate.Generator) error {
	var (
		kernelMax  uint64 = 1048576
		isRootless        = rootless.IsRootless()
		nofileSet         = false
		nprocSet          = false
	)

	if s.Rlimits == nil {
		g.Config.Process.Rlimits = nil
		return nil
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
		max := kernelMax
		current := kernelMax
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
				logrus.Warnf("failed to return RLIMIT_NOFILE ulimit %q", err)
			}
			current = rlimit.Cur
			max = rlimit.Max
		}
		g.AddProcessRlimits("RLIMIT_NOFILE", current, max)
	}
	if !nprocSet {
		max := kernelMax
		current := kernelMax
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlimit); err != nil {
				logrus.Warnf("failed to return RLIMIT_NPROC ulimit %q", err)
			}
			current = rlimit.Cur
			max = rlimit.Max
		}
		g.AddProcessRlimits("RLIMIT_NPROC", current, max)
	}

	return nil
}

// Produce the final command for the container.
func makeCommand(ctx context.Context, s *specgen.SpecGenerator, img *image.Image, rtc *config.Config) ([]string, error) {
	finalCommand := []string{}

	entrypoint := s.Entrypoint
	if len(entrypoint) == 0 && img != nil {
		newEntry, err := img.Entrypoint(ctx)
		if err != nil {
			return nil, err
		}
		entrypoint = newEntry
	}

	finalCommand = append(finalCommand, entrypoint...)

	command := s.Command
	if command == nil && img != nil {
		newCmd, err := img.Cmd(ctx)
		if err != nil {
			return nil, err
		}
		command = newCmd
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
		finalCommand = append([]string{initPath, "--"}, finalCommand...)
	}

	return finalCommand, nil
}

func SpecGenToOCI(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *image.Image, mounts []spec.Mount) (*spec.Spec, error) {
	var (
		inUserNS bool
	)
	cgroupPerm := "ro"
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")
	g.HostSpecific = true
	addCgroup := true
	canMountSys := true

	isRootless := rootless.IsRootless()
	if isRootless {
		inUserNS = true
	}
	if !s.UserNS.IsHost() {
		if s.UserNS.IsContainer() || s.UserNS.IsPath() {
			inUserNS = true
		}
		if s.UserNS.IsPrivate() {
			inUserNS = true
		}
	}
	if inUserNS && s.NetNS.IsHost() {
		canMountSys = false
	}

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
	} else if !canMountSys {
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
		nGids, err := GetAvailableGids()
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
			Type:        TypeBind,
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
	g.SetProcessCwd(s.WorkDir)

	finalCmd, err := makeCommand(ctx, s, newImage, rtc)
	if err != nil {
		return nil, err
	}
	g.SetProcessArgs(finalCmd)

	g.SetProcessTerminal(s.Terminal)

	for key, val := range s.Annotations {
		g.AddAnnotation(key, val)
	}
	g.AddProcessEnv("container", "podman")

	g.Config.Linux.Resources = s.ResourceLimits

	// Devices
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
			if err := DevicesFromPath(&g, device); err != nil {
				return nil, err
			}
		}
		// add default devices specified by caller
		for _, device := range s.Devices {
			if err := DevicesFromPath(&g, device.Path); err != nil {
				return nil, err
			}
		}
	}

	// SECURITY OPTS
	g.SetProcessNoNewPrivileges(s.NoNewPrivileges)

	if !s.Privileged {
		g.SetProcessApparmorProfile(s.ApparmorProfile)
	}

	BlockAccessToKernelFilesystems(s.Privileged, s.PidNS.IsHost(), &g)

	for name, val := range s.Env {
		g.AddProcessEnv(name, val)
	}

	if err := addRlimits(s, &g); err != nil {
		return nil, err
	}

	// NAMESPACES
	if err := specConfigureNamespaces(s, &g, rt); err != nil {
		return nil, err
	}
	configSpec := g.Config

	if err := securityConfigureGenerator(s, &g, newImage, rtc); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = SupercedeUserMounts(mounts, configSpec.Mounts)
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

	return configSpec, nil
}

func GetAvailableGids() (int64, error) {
	idMap, err := user.ParseIDMapFile("/proc/self/gid_map")
	if err != nil {
		return 0, err
	}
	count := int64(0)
	for _, r := range idMap {
		count += r.Count
	}
	return count, nil
}
