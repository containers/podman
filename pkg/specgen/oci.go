package specgen

import (
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	createconfig "github.com/containers/libpod/pkg/spec"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

func (s *SpecGenerator) ToOCISpec(rt *libpod.Runtime, newImage *image.Image) (*spec.Spec, error) {
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
		nGids, err := createconfig.GetAvailableGids()
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
			Type:        createconfig.TypeBind,
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
	g.SetProcessArgs(s.Command)
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
		if err := createconfig.AddPrivilegedDevices(&g); err != nil {
			return nil, err
		}
	} else {
		for _, device := range s.Devices {
			if err := createconfig.DevicesFromPath(&g, device.Path); err != nil {
				return nil, err
			}
		}
	}

	// SECURITY OPTS
	g.SetProcessNoNewPrivileges(s.NoNewPrivileges)

	if !s.Privileged {
		g.SetProcessApparmorProfile(s.ApparmorProfile)
	}

	createconfig.BlockAccessToKernelFilesystems(s.Privileged, s.PidNS.IsHost(), &g)

	for name, val := range s.Env {
		g.AddProcessEnv(name, val)
	}

	// TODO rlimits and ulimits needs further refinement by someone more
	// familiar with the code.
	//if err := addRlimits(config, &g); err != nil {
	//	return nil, err
	//}

	// NAMESPACES

	if err := s.pidConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := s.userConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := s.networkConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := s.utsConfigureGenerator(&g, rt); err != nil {
		return nil, err
	}

	if err := s.ipcConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := s.cgroupConfigureGenerator(&g); err != nil {
		return nil, err
	}
	configSpec := g.Config

	if err := s.securityConfigureGenerator(&g, newImage); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = createconfig.SupercedeUserMounts(s.Mounts, configSpec.Mounts)
	// Process mounts to ensure correct options
	if err := createconfig.InitFSMounts(configSpec.Mounts); err != nil {
		return nil, err
	}

	// Add annotations
	if configSpec.Annotations == nil {
		configSpec.Annotations = make(map[string]string)
	}

	// TODO cidfile is not in specgen; when wiring up cli, we will need to move this out of here
	// leaving as a reminder
	//if config.CidFile != "" {
	//	configSpec.Annotations[libpod.InspectAnnotationCIDFile] = config.CidFile
	//}

	if s.Remove {
		configSpec.Annotations[libpod.InspectAnnotationAutoremove] = libpod.InspectResponseTrue
	} else {
		configSpec.Annotations[libpod.InspectAnnotationAutoremove] = libpod.InspectResponseFalse
	}

	if len(s.VolumesFrom) > 0 {
		configSpec.Annotations[libpod.InspectAnnotationVolumesFrom] = strings.Join(s.VolumesFrom, ",")
	}

	if s.Privileged {
		configSpec.Annotations[libpod.InspectAnnotationPrivileged] = libpod.InspectResponseTrue
	} else {
		configSpec.Annotations[libpod.InspectAnnotationPrivileged] = libpod.InspectResponseFalse
	}

	// TODO Init might not make it into the specgen and therefore is not available here.  We should deal
	// with this when we wire up the CLI; leaving as a reminder
	//if s.Init {
	//	configSpec.Annotations[libpod.InspectAnnotationInit] = libpod.InspectResponseTrue
	//} else {
	//	configSpec.Annotations[libpod.InspectAnnotationInit] = libpod.InspectResponseFalse
	//}

	return configSpec, nil
}
