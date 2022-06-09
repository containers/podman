package common

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
)

func ulimits() []string {
	if !registry.IsRemote() {
		return containerConfig.Ulimits()
	}
	return nil
}

func cgroupConfig() string {
	if !registry.IsRemote() {
		return containerConfig.Cgroups()
	}
	return ""
}

func devices() []string {
	if !registry.IsRemote() {
		return containerConfig.Devices()
	}
	return nil
}

func env() []string {
	if !registry.IsRemote() {
		return containerConfig.Env()
	}
	return nil
}

func initPath() string {
	if !registry.IsRemote() {
		return containerConfig.InitPath()
	}
	return ""
}

func pidsLimit() int64 {
	if !registry.IsRemote() {
		return containerConfig.PidsLimit()
	}
	return -1
}

func policy() string {
	if !registry.IsRemote() {
		return containerConfig.Engine.PullPolicy
	}
	return ""
}

func shmSize() string {
	if !registry.IsRemote() {
		return containerConfig.ShmSize()
	}
	return ""
}

func volumes() []string {
	if !registry.IsRemote() {
		return containerConfig.Volumes()
	}
	return nil
}

func LogDriver() string {
	if !registry.IsRemote() {
		return containerConfig.Containers.LogDriver
	}
	return ""
}
