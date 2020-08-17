// +build varlink remoteclient

package varlinkapi

import (
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/pkg/rootless"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/pkg/errors"
)

//FIXME these are duplicated here to resolve a circular
//import with cmd/podman/common.
var (
	// DefaultHealthCheckInterval default value
	DefaultHealthCheckInterval = "30s"
	// DefaultHealthCheckRetries default value
	DefaultHealthCheckRetries uint = 3
	// DefaultHealthCheckStartPeriod default value
	DefaultHealthCheckStartPeriod = "0s"
	// DefaultHealthCheckTimeout default value
	DefaultHealthCheckTimeout = "30s"
	// DefaultImageVolume default value
	DefaultImageVolume = "bind"
)

// StringSliceToPtr converts a genericcliresult value into a *[]string
func StringSliceToPtr(g GenericCLIResult) *[]string {
	if !g.IsSet() {
		return nil
	}
	newT := g.Value().([]string)
	return &newT
}

// StringToPtr converts a genericcliresult value into a *string
func StringToPtr(g GenericCLIResult) *string {
	if !g.IsSet() {
		return nil
	}
	newT := g.Value().(string)
	return &newT
}

// BoolToPtr converts a genericcliresult value into a *bool
func BoolToPtr(g GenericCLIResult) *bool {
	if !g.IsSet() {
		return nil
	}
	newT := g.Value().(bool)
	return &newT
}

// AnyIntToInt64Ptr converts a genericcliresult value into an *int64
func AnyIntToInt64Ptr(g GenericCLIResult) *int64 {
	if !g.IsSet() {
		return nil
	}
	var newT int64
	switch g.Value().(type) {
	case int:
		newT = int64(g.Value().(int))
	case int64:
		newT = g.Value().(int64)
	case uint64:
		newT = int64(g.Value().(uint64))
	case uint:
		newT = int64(g.Value().(uint))
	default:
		panic(errors.Errorf("invalid int type"))
	}
	return &newT
}

// Float64ToPtr converts a genericcliresult into a *float64
func Float64ToPtr(g GenericCLIResult) *float64 {
	if !g.IsSet() {
		return nil
	}
	newT := g.Value().(float64)
	return &newT
}

// MakeVarlink creates a varlink transportable struct from GenericCLIResults
func (g GenericCLIResults) MakeVarlink() iopodman.Create {
	v := iopodman.Create{
		Args:                   g.InputArgs,
		AddHost:                StringSliceToPtr(g.Find("add-host")),
		Annotation:             StringSliceToPtr(g.Find("annotation")),
		Attach:                 StringSliceToPtr(g.Find("attach")),
		BlkioWeight:            StringToPtr(g.Find("blkio-weight")),
		BlkioWeightDevice:      StringSliceToPtr(g.Find("blkio-weight-device")),
		CapAdd:                 StringSliceToPtr(g.Find("cap-add")),
		CapDrop:                StringSliceToPtr(g.Find("cap-drop")),
		CgroupParent:           StringToPtr(g.Find("cgroup-parent")),
		CidFile:                StringToPtr(g.Find("cidfile")),
		ConmonPidfile:          StringToPtr(g.Find("conmon-pidfile")),
		CpuPeriod:              AnyIntToInt64Ptr(g.Find("cpu-period")),
		CpuQuota:               AnyIntToInt64Ptr(g.Find("cpu-quota")),
		CpuRtPeriod:            AnyIntToInt64Ptr(g.Find("cpu-rt-period")),
		CpuRtRuntime:           AnyIntToInt64Ptr(g.Find("cpu-rt-runtime")),
		CpuShares:              AnyIntToInt64Ptr(g.Find("cpu-shares")),
		Cpus:                   Float64ToPtr(g.Find("cpus")),
		CpuSetCpus:             StringToPtr(g.Find("cpuset-cpus")),
		CpuSetMems:             StringToPtr(g.Find("cpuset-mems")),
		Detach:                 BoolToPtr(g.Find("detach")),
		DetachKeys:             StringToPtr(g.Find("detach-keys")),
		Device:                 StringSliceToPtr(g.Find("device")),
		DeviceReadBps:          StringSliceToPtr(g.Find("device-read-bps")),
		DeviceReadIops:         StringSliceToPtr(g.Find("device-read-iops")),
		DeviceWriteBps:         StringSliceToPtr(g.Find("device-write-bps")),
		DeviceWriteIops:        StringSliceToPtr(g.Find("device-write-iops")),
		Dns:                    StringSliceToPtr(g.Find("dns")),
		DnsOpt:                 StringSliceToPtr(g.Find("dns-opt")),
		DnsSearch:              StringSliceToPtr(g.Find("dns-search")),
		Entrypoint:             StringToPtr(g.Find("entrypoint")),
		Env:                    StringSliceToPtr(g.Find("env")),
		EnvFile:                StringSliceToPtr(g.Find("env-file")),
		Expose:                 StringSliceToPtr(g.Find("expose")),
		Gidmap:                 StringSliceToPtr(g.Find("gidmap")),
		Groupadd:               StringSliceToPtr(g.Find("group-add")),
		HealthcheckCommand:     StringToPtr(g.Find("healthcheck-command")),
		HealthcheckInterval:    StringToPtr(g.Find("healthcheck-interval")),
		HealthcheckRetries:     AnyIntToInt64Ptr(g.Find("healthcheck-retries")),
		HealthcheckStartPeriod: StringToPtr(g.Find("healthcheck-start-period")),
		HealthcheckTimeout:     StringToPtr(g.Find("healthcheck-timeout")),
		Hostname:               StringToPtr(g.Find("hostname")),
		ImageVolume:            StringToPtr(g.Find("image-volume")),
		Init:                   BoolToPtr(g.Find("init")),
		InitPath:               StringToPtr(g.Find("init-path")),
		Interactive:            BoolToPtr(g.Find("interactive")),
		Ip:                     StringToPtr(g.Find("ip")),
		Ipc:                    StringToPtr(g.Find("ipc")),
		KernelMemory:           StringToPtr(g.Find("kernel-memory")),
		Label:                  StringSliceToPtr(g.Find("label")),
		LabelFile:              StringSliceToPtr(g.Find("label-file")),
		LogDriver:              StringToPtr(g.Find("log-driver")),
		LogOpt:                 StringSliceToPtr(g.Find("log-opt")),
		MacAddress:             StringToPtr(g.Find("mac-address")),
		Memory:                 StringToPtr(g.Find("memory")),
		MemoryReservation:      StringToPtr(g.Find("memory-reservation")),
		MemorySwap:             StringToPtr(g.Find("memory-swap")),
		MemorySwappiness:       AnyIntToInt64Ptr(g.Find("memory-swappiness")),
		Name:                   StringToPtr(g.Find("name")),
		Network:                StringToPtr(g.Find("network")),
		OomKillDisable:         BoolToPtr(g.Find("oom-kill-disable")),
		OomScoreAdj:            AnyIntToInt64Ptr(g.Find("oom-score-adj")),
		OverrideOS:             StringToPtr(g.Find("override-os")),
		OverrideArch:           StringToPtr(g.Find("override-arch")),
		Pid:                    StringToPtr(g.Find("pid")),
		PidsLimit:              AnyIntToInt64Ptr(g.Find("pids-limit")),
		Pod:                    StringToPtr(g.Find("pod")),
		Privileged:             BoolToPtr(g.Find("privileged")),
		Publish:                StringSliceToPtr(g.Find("publish")),
		PublishAll:             BoolToPtr(g.Find("publish-all")),
		Pull:                   StringToPtr(g.Find("pull")),
		Quiet:                  BoolToPtr(g.Find("quiet")),
		Readonly:               BoolToPtr(g.Find("read-only")),
		Readonlytmpfs:          BoolToPtr(g.Find("read-only-tmpfs")),
		Restart:                StringToPtr(g.Find("restart")),
		Rm:                     BoolToPtr(g.Find("rm")),
		Rootfs:                 BoolToPtr(g.Find("rootfs")),
		SecurityOpt:            StringSliceToPtr(g.Find("security-opt")),
		ShmSize:                StringToPtr(g.Find("shm-size")),
		StopSignal:             StringToPtr(g.Find("stop-signal")),
		StopTimeout:            AnyIntToInt64Ptr(g.Find("stop-timeout")),
		StorageOpt:             StringSliceToPtr(g.Find("storage-opt")),
		Subuidname:             StringToPtr(g.Find("subuidname")),
		Subgidname:             StringToPtr(g.Find("subgidname")),
		Sysctl:                 StringSliceToPtr(g.Find("sysctl")),
		Systemd:                StringToPtr(g.Find("systemd")),
		Tmpfs:                  StringSliceToPtr(g.Find("tmpfs")),
		Tty:                    BoolToPtr(g.Find("tty")),
		Uidmap:                 StringSliceToPtr(g.Find("uidmap")),
		Ulimit:                 StringSliceToPtr(g.Find("ulimit")),
		User:                   StringToPtr(g.Find("user")),
		Userns:                 StringToPtr(g.Find("userns")),
		Uts:                    StringToPtr(g.Find("uts")),
		Mount:                  StringSliceToPtr(g.Find("mount")),
		Volume:                 StringSliceToPtr(g.Find("volume")),
		VolumesFrom:            StringSliceToPtr(g.Find("volumes-from")),
		WorkDir:                StringToPtr(g.Find("workdir")),
	}

	return v
}

func stringSliceFromVarlink(v *[]string, flagName string, defaultValue *[]string) CRStringSlice {
	cr := CRStringSlice{}
	if v == nil {
		cr.Val = []string{}
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func stringFromVarlink(v *string, flagName string, defaultValue *string) CRString {
	cr := CRString{}
	if v == nil {
		cr.Val = ""
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func boolFromVarlink(v *bool, flagName string, defaultValue bool) CRBool {
	cr := CRBool{}
	if v == nil {
		// In case a cli bool default value is true
		cr.Val = defaultValue
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func uint64FromVarlink(v *int64, flagName string, defaultValue *uint64) CRUint64 {
	cr := CRUint64{}
	if v == nil {
		cr.Val = 0
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = uint64(*v)
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func int64FromVarlink(v *int64, flagName string, defaultValue *int64) CRInt64 {
	cr := CRInt64{}
	if v == nil {
		cr.Val = 0
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func float64FromVarlink(v *float64, flagName string, defaultValue *float64) CRFloat64 {
	cr := CRFloat64{}
	if v == nil {
		cr.Val = 0
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func uintFromVarlink(v *int64, flagName string, defaultValue *uint) CRUint {
	cr := CRUint{}
	if v == nil {
		cr.Val = 0
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = uint(*v)
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func stringArrayFromVarlink(v *[]string, flagName string, defaultValue *[]string) CRStringArray {
	cr := CRStringArray{}
	if v == nil {
		cr.Val = []string{}
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Changed = false
	} else {
		cr.Val = *v
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

func intFromVarlink(v *int64, flagName string, defaultValue *int) CRInt {
	cr := CRInt{}
	if v == nil {
		if defaultValue != nil {
			cr.Val = *defaultValue
		}
		cr.Val = 0
		cr.Changed = false
	} else {
		cr.Val = int(*v)
		cr.Changed = true
	}
	cr.Flag = flagName
	return cr
}

// VarlinkCreateToGeneric creates a GenericCLIResults from the varlink create
// structure.
func VarlinkCreateToGeneric(opts iopodman.Create) GenericCLIResults {
	// FIXME this will need to be fixed!!!!! With containers conf
	//containerConfig := cliconfig.GetDefaultConfig()
	// TODO | WARN
	// We do not get a default network over varlink. Unlike the other default values for some cli
	// elements, it seems it gets set to the default anyway.

	var memSwapDefault int64 = -1
	netModeDefault := "bridge"
	systemdDefault := "true"
	if rootless.IsRootless() {
		netModeDefault = "slirp4netns"
	}

	shmSize := config.DefaultShmSize

	m := make(map[string]GenericCLIResult)
	m["add-host"] = stringSliceFromVarlink(opts.AddHost, "add-host", nil)
	m["annotation"] = stringSliceFromVarlink(opts.Annotation, "annotation", nil)
	m["attach"] = stringSliceFromVarlink(opts.Attach, "attach", nil)
	m["blkio-weight"] = stringFromVarlink(opts.BlkioWeight, "blkio-weight", nil)
	m["blkio-weight-device"] = stringSliceFromVarlink(opts.BlkioWeightDevice, "blkio-weight-device", nil)
	m["cap-add"] = stringSliceFromVarlink(opts.CapAdd, "cap-add", nil)
	m["cap-drop"] = stringSliceFromVarlink(opts.CapDrop, "cap-drop", nil)
	m["cgroup-parent"] = stringFromVarlink(opts.CgroupParent, "cgroup-parent", nil)
	m["cidfile"] = stringFromVarlink(opts.CidFile, "cidfile", nil)
	m["conmon-pidfile"] = stringFromVarlink(opts.ConmonPidfile, "conmon-file", nil)
	m["cpu-period"] = uint64FromVarlink(opts.CpuPeriod, "cpu-period", nil)
	m["cpu-quota"] = int64FromVarlink(opts.CpuQuota, "quota", nil)
	m["cpu-rt-period"] = uint64FromVarlink(opts.CpuRtPeriod, "cpu-rt-period", nil)
	m["cpu-rt-runtime"] = int64FromVarlink(opts.CpuRtRuntime, "cpu-rt-quota", nil)
	m["cpu-shares"] = uint64FromVarlink(opts.CpuShares, "cpu-shares", nil)
	m["cpus"] = float64FromVarlink(opts.Cpus, "cpus", nil)
	m["cpuset-cpus"] = stringFromVarlink(opts.CpuSetCpus, "cpuset-cpus", nil)
	m["cpuset-mems"] = stringFromVarlink(opts.CpuSetMems, "cpuset-mems", nil)
	m["detach"] = boolFromVarlink(opts.Detach, "detach", false)
	m["detach-keys"] = stringFromVarlink(opts.DetachKeys, "detach-keys", nil)
	m["device"] = stringSliceFromVarlink(opts.Device, "device", nil)
	m["device-read-bps"] = stringSliceFromVarlink(opts.DeviceReadBps, "device-read-bps", nil)
	m["device-read-iops"] = stringSliceFromVarlink(opts.DeviceReadIops, "device-read-iops", nil)
	m["device-write-bps"] = stringSliceFromVarlink(opts.DeviceWriteBps, "write-device-bps", nil)
	m["device-write-iops"] = stringSliceFromVarlink(opts.DeviceWriteIops, "write-device-iops", nil)
	m["dns"] = stringSliceFromVarlink(opts.Dns, "dns", nil)
	m["dns-opt"] = stringSliceFromVarlink(opts.DnsOpt, "dns-opt", nil)
	m["dns-search"] = stringSliceFromVarlink(opts.DnsSearch, "dns-search", nil)
	m["entrypoint"] = stringFromVarlink(opts.Entrypoint, "entrypoint", nil)
	m["env"] = stringArrayFromVarlink(opts.Env, "env", nil)
	m["env-file"] = stringSliceFromVarlink(opts.EnvFile, "env-file", nil)
	m["expose"] = stringSliceFromVarlink(opts.Expose, "expose", nil)
	m["gidmap"] = stringSliceFromVarlink(opts.Gidmap, "gidmap", nil)
	m["group-add"] = stringSliceFromVarlink(opts.Groupadd, "group-add", nil)
	m["healthcheck-command"] = stringFromVarlink(opts.HealthcheckCommand, "healthcheck-command", nil)
	m["healthcheck-interval"] = stringFromVarlink(opts.HealthcheckInterval, "healthcheck-interval", &DefaultHealthCheckInterval)
	m["healthcheck-retries"] = uintFromVarlink(opts.HealthcheckRetries, "healthcheck-retries", &DefaultHealthCheckRetries)
	m["healthcheck-start-period"] = stringFromVarlink(opts.HealthcheckStartPeriod, "healthcheck-start-period", &DefaultHealthCheckStartPeriod)
	m["healthcheck-timeout"] = stringFromVarlink(opts.HealthcheckTimeout, "healthcheck-timeout", &DefaultHealthCheckTimeout)
	m["hostname"] = stringFromVarlink(opts.Hostname, "hostname", nil)
	m["image-volume"] = stringFromVarlink(opts.ImageVolume, "image-volume", &DefaultImageVolume)
	m["init"] = boolFromVarlink(opts.Init, "init", false)
	m["init-path"] = stringFromVarlink(opts.InitPath, "init-path", nil)
	m["interactive"] = boolFromVarlink(opts.Interactive, "interactive", false)
	m["ip"] = stringFromVarlink(opts.Ip, "ip", nil)
	m["ipc"] = stringFromVarlink(opts.Ipc, "ipc", nil)
	m["kernel-memory"] = stringFromVarlink(opts.KernelMemory, "kernel-memory", nil)
	m["label"] = stringArrayFromVarlink(opts.Label, "label", nil)
	m["label-file"] = stringSliceFromVarlink(opts.LabelFile, "label-file", nil)
	m["log-driver"] = stringFromVarlink(opts.LogDriver, "log-driver", nil)
	m["log-opt"] = stringSliceFromVarlink(opts.LogOpt, "log-opt", nil)
	m["mac-address"] = stringFromVarlink(opts.MacAddress, "mac-address", nil)
	m["memory"] = stringFromVarlink(opts.Memory, "memory", nil)
	m["memory-reservation"] = stringFromVarlink(opts.MemoryReservation, "memory-reservation", nil)
	m["memory-swap"] = stringFromVarlink(opts.MemorySwap, "memory-swap", nil)
	m["memory-swappiness"] = int64FromVarlink(opts.MemorySwappiness, "memory-swappiness", &memSwapDefault)
	m["name"] = stringFromVarlink(opts.Name, "name", nil)
	m["network"] = stringFromVarlink(opts.Network, "network", &netModeDefault)
	m["no-hosts"] = boolFromVarlink(opts.NoHosts, "no-hosts", false)
	m["oom-kill-disable"] = boolFromVarlink(opts.OomKillDisable, "oon-kill-disable", false)
	m["oom-score-adj"] = intFromVarlink(opts.OomScoreAdj, "oom-score-adj", nil)
	m["override-os"] = stringFromVarlink(opts.OverrideOS, "override-os", nil)
	m["override-arch"] = stringFromVarlink(opts.OverrideArch, "override-arch", nil)
	m["pid"] = stringFromVarlink(opts.Pid, "pid", nil)
	m["pids-limit"] = int64FromVarlink(opts.PidsLimit, "pids-limit", nil)
	m["pod"] = stringFromVarlink(opts.Pod, "pod", nil)
	m["privileged"] = boolFromVarlink(opts.Privileged, "privileged", false)
	m["publish"] = stringSliceFromVarlink(opts.Publish, "publish", nil)
	m["publish-all"] = boolFromVarlink(opts.PublishAll, "publish-all", false)
	m["pull"] = stringFromVarlink(opts.Pull, "missing", nil)
	m["quiet"] = boolFromVarlink(opts.Quiet, "quiet", false)
	m["read-only"] = boolFromVarlink(opts.Readonly, "read-only", false)
	m["read-only-tmpfs"] = boolFromVarlink(opts.Readonlytmpfs, "read-only-tmpfs", true)
	m["restart"] = stringFromVarlink(opts.Restart, "restart", nil)
	m["rm"] = boolFromVarlink(opts.Rm, "rm", false)
	m["rootfs"] = boolFromVarlink(opts.Rootfs, "rootfs", false)
	m["security-opt"] = stringArrayFromVarlink(opts.SecurityOpt, "security-opt", nil)
	m["shm-size"] = stringFromVarlink(opts.ShmSize, "shm-size", &shmSize)
	m["stop-signal"] = stringFromVarlink(opts.StopSignal, "stop-signal", nil)
	m["stop-timeout"] = uintFromVarlink(opts.StopTimeout, "stop-timeout", nil)
	m["storage-opt"] = stringSliceFromVarlink(opts.StorageOpt, "storage-opt", nil)
	m["subgidname"] = stringFromVarlink(opts.Subgidname, "subgidname", nil)
	m["subuidname"] = stringFromVarlink(opts.Subuidname, "subuidname", nil)
	m["sysctl"] = stringSliceFromVarlink(opts.Sysctl, "sysctl", nil)
	m["systemd"] = stringFromVarlink(opts.Systemd, "systemd", &systemdDefault)
	m["tmpfs"] = stringSliceFromVarlink(opts.Tmpfs, "tmpfs", nil)
	m["tty"] = boolFromVarlink(opts.Tty, "tty", false)
	m["uidmap"] = stringSliceFromVarlink(opts.Uidmap, "uidmap", nil)
	m["ulimit"] = stringSliceFromVarlink(opts.Ulimit, "ulimit", nil)
	m["user"] = stringFromVarlink(opts.User, "user", nil)
	m["userns"] = stringFromVarlink(opts.Userns, "userns", nil)
	m["uts"] = stringFromVarlink(opts.Uts, "uts", nil)
	m["mount"] = stringArrayFromVarlink(opts.Mount, "mount", nil)
	m["volume"] = stringArrayFromVarlink(opts.Volume, "volume", nil)
	m["volumes-from"] = stringSliceFromVarlink(opts.VolumesFrom, "volumes-from", nil)
	m["workdir"] = stringFromVarlink(opts.WorkDir, "workdir", nil)

	gcli := GenericCLIResults{m, opts.Args}
	return gcli
}

// Find returns a flag from a GenericCLIResults by name
func (g GenericCLIResults) Find(name string) GenericCLIResult {
	result, ok := g.results[name]
	if ok {
		return result
	}
	panic(errors.Errorf("unable to find generic flag for varlink %s", name))
}
