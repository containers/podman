package common

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func ulimits() []string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Ulimits()
	}
	return nil
}

func cgroupConfig() string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Cgroups()
	}
	return ""
}

func devices() []string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Devices()
	}
	return nil
}

func Env() []string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Env()
	}
	return nil
}

func pidsLimit() int64 {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.PidsLimit()
	}
	return -1
}

func policy() string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Engine.PullPolicy
	}
	return ""
}

func shmSize() string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.ShmSize()
	}
	return ""
}

func volumes() []string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Volumes()
	}
	return nil
}

func LogDriver() string {
	if !registry.IsRemote() {
		return registry.PodmanConfig().ContainersConfDefaultsRO.Containers.LogDriver
	}
	return ""
}

// DefineCreateDefault is used to initialize ctr create options before flag initialization
func DefineCreateDefaults(opts *entities.ContainerCreateOptions) {
	opts.LogDriver = LogDriver()
	opts.CgroupsMode = cgroupConfig()
	opts.MemorySwappiness = -1
	opts.ImageVolume = registry.PodmanConfig().ContainersConfDefaultsRO.Engine.ImageVolumeMode
	opts.Pull = policy()
	opts.ReadOnlyTmpFS = true
	opts.SdNotifyMode = define.SdNotifyModeContainer
	opts.StopTimeout = registry.PodmanConfig().ContainersConfDefaultsRO.Engine.StopTimeout
	opts.Systemd = "true"
	opts.Timezone = registry.PodmanConfig().ContainersConfDefaultsRO.TZ()
	opts.Umask = registry.PodmanConfig().ContainersConfDefaultsRO.Umask()
	opts.Ulimit = ulimits()
	opts.SeccompPolicy = "default"
	opts.Volume = volumes()
}
