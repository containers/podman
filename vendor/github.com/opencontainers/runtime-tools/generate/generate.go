// Package generate implements functions generating container config files.
package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate/seccomp"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/syndtr/gocapability/capability"
)

var (
	// Namespaces include the names of supported namespaces.
	Namespaces = []string{"network", "pid", "mount", "ipc", "uts", "user", "cgroup"}
)

// Generator represents a generator for a container spec.
type Generator struct {
	spec         *rspec.Spec
	HostSpecific bool
}

// ExportOptions have toggles for exporting only certain parts of the specification
type ExportOptions struct {
	Seccomp bool // seccomp toggles if only seccomp should be exported
}

// New creates a spec Generator with the default spec.
func New() Generator {
	spec := rspec.Spec{
		Version: rspec.Version,
		Root: &rspec.Root{
			Path:     "",
			Readonly: false,
		},
		Process: &rspec.Process{
			Terminal: false,
			User:     rspec.User{},
			Args: []string{
				"sh",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd: "/",
			Capabilities: &rspec.LinuxCapabilities{
				Bounding: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Permitted: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Inheritable: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Effective: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Ambient: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
			},
			Rlimits: []rspec.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
		},
		Hostname: "mrsdalloway",
		Mounts: []rspec.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     nil,
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
		},
		Linux: &rspec.Linux{
			Resources: &rspec.LinuxResources{
				Devices: []rspec.LinuxDeviceCgroup{
					{
						Allow:  false,
						Access: "rwm",
					},
				},
			},
			Namespaces: []rspec.LinuxNamespace{
				{
					Type: "pid",
				},
				{
					Type: "network",
				},
				{
					Type: "ipc",
				},
				{
					Type: "uts",
				},
				{
					Type: "mount",
				},
			},
			Devices: []rspec.LinuxDevice{},
		},
	}
	spec.Linux.Seccomp = seccomp.DefaultProfile(&spec)
	return Generator{
		spec: &spec,
	}
}

// NewFromSpec creates a spec Generator from a given spec.
func NewFromSpec(spec *rspec.Spec) Generator {
	return Generator{
		spec: spec,
	}
}

// NewFromFile loads the template specified in a file into a spec Generator.
func NewFromFile(path string) (Generator, error) {
	cf, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Generator{}, fmt.Errorf("template configuration at %s not found", path)
		}
	}
	defer cf.Close()

	return NewFromTemplate(cf)
}

// NewFromTemplate loads the template from io.Reader into a spec Generator.
func NewFromTemplate(r io.Reader) (Generator, error) {
	var spec rspec.Spec
	if err := json.NewDecoder(r).Decode(&spec); err != nil {
		return Generator{}, err
	}
	return Generator{
		spec: &spec,
	}, nil
}

// SetSpec sets the spec in the Generator g.
func (g *Generator) SetSpec(spec *rspec.Spec) {
	g.spec = spec
}

// Spec gets the spec in the Generator g.
func (g *Generator) Spec() *rspec.Spec {
	return g.spec
}

// Save writes the spec into w.
func (g *Generator) Save(w io.Writer, exportOpts ExportOptions) (err error) {
	var data []byte

	if g.spec.Linux != nil {
		buf, err := json.Marshal(g.spec.Linux)
		if err != nil {
			return err
		}
		if string(buf) == "{}" {
			g.spec.Linux = nil
		}
	}

	if exportOpts.Seccomp {
		data, err = json.MarshalIndent(g.spec.Linux.Seccomp, "", "\t")
	} else {
		data, err = json.MarshalIndent(g.spec, "", "\t")
	}
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// SaveToFile writes the spec into a file.
func (g *Generator) SaveToFile(path string, exportOpts ExportOptions) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return g.Save(f, exportOpts)
}

// SetVersion sets g.spec.Version.
func (g *Generator) SetVersion(version string) {
	g.initSpec()
	g.spec.Version = version
}

// SetRootPath sets g.spec.Root.Path.
func (g *Generator) SetRootPath(path string) {
	g.initSpecRoot()
	g.spec.Root.Path = path
}

// SetRootReadonly sets g.spec.Root.Readonly.
func (g *Generator) SetRootReadonly(b bool) {
	g.initSpecRoot()
	g.spec.Root.Readonly = b
}

// SetHostname sets g.spec.Hostname.
func (g *Generator) SetHostname(s string) {
	g.initSpec()
	g.spec.Hostname = s
}

// ClearAnnotations clears g.spec.Annotations.
func (g *Generator) ClearAnnotations() {
	if g.spec == nil {
		return
	}
	g.spec.Annotations = make(map[string]string)
}

// AddAnnotation adds an annotation into g.spec.Annotations.
func (g *Generator) AddAnnotation(key, value string) {
	g.initSpecAnnotations()
	g.spec.Annotations[key] = value
}

// RemoveAnnotation remove an annotation from g.spec.Annotations.
func (g *Generator) RemoveAnnotation(key string) {
	if g.spec == nil || g.spec.Annotations == nil {
		return
	}
	delete(g.spec.Annotations, key)
}

// SetProcessConsoleSize sets g.spec.Process.ConsoleSize.
func (g *Generator) SetProcessConsoleSize(width, height uint) {
	g.initSpecProcessConsoleSize()
	g.spec.Process.ConsoleSize.Width = width
	g.spec.Process.ConsoleSize.Height = height
}

// SetProcessUID sets g.spec.Process.User.UID.
func (g *Generator) SetProcessUID(uid uint32) {
	g.initSpecProcess()
	g.spec.Process.User.UID = uid
}

// SetProcessGID sets g.spec.Process.User.GID.
func (g *Generator) SetProcessGID(gid uint32) {
	g.initSpecProcess()
	g.spec.Process.User.GID = gid
}

// SetProcessCwd sets g.spec.Process.Cwd.
func (g *Generator) SetProcessCwd(cwd string) {
	g.initSpecProcess()
	g.spec.Process.Cwd = cwd
}

// SetProcessNoNewPrivileges sets g.spec.Process.NoNewPrivileges.
func (g *Generator) SetProcessNoNewPrivileges(b bool) {
	g.initSpecProcess()
	g.spec.Process.NoNewPrivileges = b
}

// SetProcessTerminal sets g.spec.Process.Terminal.
func (g *Generator) SetProcessTerminal(b bool) {
	g.initSpecProcess()
	g.spec.Process.Terminal = b
}

// SetProcessApparmorProfile sets g.spec.Process.ApparmorProfile.
func (g *Generator) SetProcessApparmorProfile(prof string) {
	g.initSpecProcess()
	g.spec.Process.ApparmorProfile = prof
}

// SetProcessArgs sets g.spec.Process.Args.
func (g *Generator) SetProcessArgs(args []string) {
	g.initSpecProcess()
	g.spec.Process.Args = args
}

// ClearProcessEnv clears g.spec.Process.Env.
func (g *Generator) ClearProcessEnv() {
	if g.spec == nil {
		return
	}
	g.spec.Process.Env = []string{}
}

// AddProcessEnv adds name=value into g.spec.Process.Env, or replaces an
// existing entry with the given name.
func (g *Generator) AddProcessEnv(name, value string) {
	g.initSpecProcess()

	env := fmt.Sprintf("%s=%s", name, value)
	for idx := range g.spec.Process.Env {
		if strings.HasPrefix(g.spec.Process.Env[idx], name+"=") {
			g.spec.Process.Env[idx] = env
			return
		}
	}
	g.spec.Process.Env = append(g.spec.Process.Env, env)
}

// AddProcessRlimits adds rlimit into g.spec.Process.Rlimits.
func (g *Generator) AddProcessRlimits(rType string, rHard uint64, rSoft uint64) {
	g.initSpecProcess()
	for i, rlimit := range g.spec.Process.Rlimits {
		if rlimit.Type == rType {
			g.spec.Process.Rlimits[i].Hard = rHard
			g.spec.Process.Rlimits[i].Soft = rSoft
			return
		}
	}

	newRlimit := rspec.POSIXRlimit{
		Type: rType,
		Hard: rHard,
		Soft: rSoft,
	}
	g.spec.Process.Rlimits = append(g.spec.Process.Rlimits, newRlimit)
}

// RemoveProcessRlimits removes a rlimit from g.spec.Process.Rlimits.
func (g *Generator) RemoveProcessRlimits(rType string) error {
	if g.spec == nil {
		return nil
	}
	for i, rlimit := range g.spec.Process.Rlimits {
		if rlimit.Type == rType {
			g.spec.Process.Rlimits = append(g.spec.Process.Rlimits[:i], g.spec.Process.Rlimits[i+1:]...)
			return nil
		}
	}
	return nil
}

// ClearProcessRlimits clear g.spec.Process.Rlimits.
func (g *Generator) ClearProcessRlimits() {
	if g.spec == nil {
		return
	}
	g.spec.Process.Rlimits = []rspec.POSIXRlimit{}
}

// ClearProcessAdditionalGids clear g.spec.Process.AdditionalGids.
func (g *Generator) ClearProcessAdditionalGids() {
	if g.spec == nil {
		return
	}
	g.spec.Process.User.AdditionalGids = []uint32{}
}

// AddProcessAdditionalGid adds an additional gid into g.spec.Process.AdditionalGids.
func (g *Generator) AddProcessAdditionalGid(gid uint32) {
	g.initSpecProcess()
	for _, group := range g.spec.Process.User.AdditionalGids {
		if group == gid {
			return
		}
	}
	g.spec.Process.User.AdditionalGids = append(g.spec.Process.User.AdditionalGids, gid)
}

// SetProcessSelinuxLabel sets g.spec.Process.SelinuxLabel.
func (g *Generator) SetProcessSelinuxLabel(label string) {
	g.initSpecProcess()
	g.spec.Process.SelinuxLabel = label
}

// SetLinuxCgroupsPath sets g.spec.Linux.CgroupsPath.
func (g *Generator) SetLinuxCgroupsPath(path string) {
	g.initSpecLinux()
	g.spec.Linux.CgroupsPath = path
}

// SetLinuxMountLabel sets g.spec.Linux.MountLabel.
func (g *Generator) SetLinuxMountLabel(label string) {
	g.initSpecLinux()
	g.spec.Linux.MountLabel = label
}

// SetProcessOOMScoreAdj sets g.spec.Process.OOMScoreAdj.
func (g *Generator) SetProcessOOMScoreAdj(adj int) {
	g.initSpecProcess()
	g.spec.Process.OOMScoreAdj = &adj
}

// SetLinuxResourcesCPUShares sets g.spec.Linux.Resources.CPU.Shares.
func (g *Generator) SetLinuxResourcesCPUShares(shares uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Shares = &shares
}

// SetLinuxResourcesCPUQuota sets g.spec.Linux.Resources.CPU.Quota.
func (g *Generator) SetLinuxResourcesCPUQuota(quota int64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Quota = &quota
}

// SetLinuxResourcesCPUPeriod sets g.spec.Linux.Resources.CPU.Period.
func (g *Generator) SetLinuxResourcesCPUPeriod(period uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Period = &period
}

// SetLinuxResourcesCPURealtimeRuntime sets g.spec.Linux.Resources.CPU.RealtimeRuntime.
func (g *Generator) SetLinuxResourcesCPURealtimeRuntime(time int64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.RealtimeRuntime = &time
}

// SetLinuxResourcesCPURealtimePeriod sets g.spec.Linux.Resources.CPU.RealtimePeriod.
func (g *Generator) SetLinuxResourcesCPURealtimePeriod(period uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.RealtimePeriod = &period
}

// SetLinuxResourcesCPUCpus sets g.spec.Linux.Resources.CPU.Cpus.
func (g *Generator) SetLinuxResourcesCPUCpus(cpus string) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Cpus = cpus
}

// SetLinuxResourcesCPUMems sets g.spec.Linux.Resources.CPU.Mems.
func (g *Generator) SetLinuxResourcesCPUMems(mems string) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Mems = mems
}

// AddLinuxResourcesHugepageLimit adds or sets g.spec.Linux.Resources.HugepageLimits.
func (g *Generator) AddLinuxResourcesHugepageLimit(pageSize string, limit uint64) {
	hugepageLimit := rspec.LinuxHugepageLimit{
		Pagesize: pageSize,
		Limit:    limit,
	}

	g.initSpecLinuxResources()
	for i, pageLimit := range g.spec.Linux.Resources.HugepageLimits {
		if pageLimit.Pagesize == pageSize {
			g.spec.Linux.Resources.HugepageLimits[i].Limit = limit
			return
		}
	}
	g.spec.Linux.Resources.HugepageLimits = append(g.spec.Linux.Resources.HugepageLimits, hugepageLimit)
}

// DropLinuxResourcesHugepageLimit drops a hugepage limit from g.spec.Linux.Resources.HugepageLimits.
func (g *Generator) DropLinuxResourcesHugepageLimit(pageSize string) error {
	g.initSpecLinuxResources()
	for i, pageLimit := range g.spec.Linux.Resources.HugepageLimits {
		if pageLimit.Pagesize == pageSize {
			g.spec.Linux.Resources.HugepageLimits = append(g.spec.Linux.Resources.HugepageLimits[:i], g.spec.Linux.Resources.HugepageLimits[i+1:]...)
			return nil
		}
	}

	return nil
}

// SetLinuxResourcesMemoryLimit sets g.spec.Linux.Resources.Memory.Limit.
func (g *Generator) SetLinuxResourcesMemoryLimit(limit int64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Limit = &limit
}

// SetLinuxResourcesMemoryReservation sets g.spec.Linux.Resources.Memory.Reservation.
func (g *Generator) SetLinuxResourcesMemoryReservation(reservation int64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Reservation = &reservation
}

// SetLinuxResourcesMemorySwap sets g.spec.Linux.Resources.Memory.Swap.
func (g *Generator) SetLinuxResourcesMemorySwap(swap int64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Swap = &swap
}

// SetLinuxResourcesMemoryKernel sets g.spec.Linux.Resources.Memory.Kernel.
func (g *Generator) SetLinuxResourcesMemoryKernel(kernel int64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Kernel = &kernel
}

// SetLinuxResourcesMemoryKernelTCP sets g.spec.Linux.Resources.Memory.KernelTCP.
func (g *Generator) SetLinuxResourcesMemoryKernelTCP(kernelTCP int64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.KernelTCP = &kernelTCP
}

// SetLinuxResourcesMemorySwappiness sets g.spec.Linux.Resources.Memory.Swappiness.
func (g *Generator) SetLinuxResourcesMemorySwappiness(swappiness uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Swappiness = &swappiness
}

// SetLinuxResourcesMemoryDisableOOMKiller sets g.spec.Linux.Resources.Memory.DisableOOMKiller.
func (g *Generator) SetLinuxResourcesMemoryDisableOOMKiller(disable bool) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.DisableOOMKiller = &disable
}

// SetLinuxResourcesNetworkClassID sets g.spec.Linux.Resources.Network.ClassID.
func (g *Generator) SetLinuxResourcesNetworkClassID(classid uint32) {
	g.initSpecLinuxResourcesNetwork()
	g.spec.Linux.Resources.Network.ClassID = &classid
}

// AddLinuxResourcesNetworkPriorities adds or sets g.spec.Linux.Resources.Network.Priorities.
func (g *Generator) AddLinuxResourcesNetworkPriorities(name string, prio uint32) {
	g.initSpecLinuxResourcesNetwork()
	for i, netPriority := range g.spec.Linux.Resources.Network.Priorities {
		if netPriority.Name == name {
			g.spec.Linux.Resources.Network.Priorities[i].Priority = prio
			return
		}
	}
	interfacePrio := new(rspec.LinuxInterfacePriority)
	interfacePrio.Name = name
	interfacePrio.Priority = prio
	g.spec.Linux.Resources.Network.Priorities = append(g.spec.Linux.Resources.Network.Priorities, *interfacePrio)
}

// DropLinuxResourcesNetworkPriorities drops one item from g.spec.Linux.Resources.Network.Priorities.
func (g *Generator) DropLinuxResourcesNetworkPriorities(name string) {
	g.initSpecLinuxResourcesNetwork()
	for i, netPriority := range g.spec.Linux.Resources.Network.Priorities {
		if netPriority.Name == name {
			g.spec.Linux.Resources.Network.Priorities = append(g.spec.Linux.Resources.Network.Priorities[:i], g.spec.Linux.Resources.Network.Priorities[i+1:]...)
			return
		}
	}
}

// SetLinuxResourcesPidsLimit sets g.spec.Linux.Resources.Pids.Limit.
func (g *Generator) SetLinuxResourcesPidsLimit(limit int64) {
	g.initSpecLinuxResourcesPids()
	g.spec.Linux.Resources.Pids.Limit = limit
}

// ClearLinuxSysctl clears g.spec.Linux.Sysctl.
func (g *Generator) ClearLinuxSysctl() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.Sysctl = make(map[string]string)
}

// AddLinuxSysctl adds a new sysctl config into g.spec.Linux.Sysctl.
func (g *Generator) AddLinuxSysctl(key, value string) {
	g.initSpecLinuxSysctl()
	g.spec.Linux.Sysctl[key] = value
}

// RemoveLinuxSysctl removes a sysctl config from g.spec.Linux.Sysctl.
func (g *Generator) RemoveLinuxSysctl(key string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Sysctl == nil {
		return
	}
	delete(g.spec.Linux.Sysctl, key)
}

// ClearLinuxUIDMappings clear g.spec.Linux.UIDMappings.
func (g *Generator) ClearLinuxUIDMappings() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.UIDMappings = []rspec.LinuxIDMapping{}
}

// AddLinuxUIDMapping adds uidMap into g.spec.Linux.UIDMappings.
func (g *Generator) AddLinuxUIDMapping(hid, cid, size uint32) {
	idMapping := rspec.LinuxIDMapping{
		HostID:      hid,
		ContainerID: cid,
		Size:        size,
	}

	g.initSpecLinux()
	g.spec.Linux.UIDMappings = append(g.spec.Linux.UIDMappings, idMapping)
}

// ClearLinuxGIDMappings clear g.spec.Linux.GIDMappings.
func (g *Generator) ClearLinuxGIDMappings() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.GIDMappings = []rspec.LinuxIDMapping{}
}

// AddLinuxGIDMapping adds gidMap into g.spec.Linux.GIDMappings.
func (g *Generator) AddLinuxGIDMapping(hid, cid, size uint32) {
	idMapping := rspec.LinuxIDMapping{
		HostID:      hid,
		ContainerID: cid,
		Size:        size,
	}

	g.initSpecLinux()
	g.spec.Linux.GIDMappings = append(g.spec.Linux.GIDMappings, idMapping)
}

// SetLinuxRootPropagation sets g.spec.Linux.RootfsPropagation.
func (g *Generator) SetLinuxRootPropagation(rp string) error {
	switch rp {
	case "":
	case "private":
	case "rprivate":
	case "slave":
	case "rslave":
	case "shared":
	case "rshared":
	default:
		return fmt.Errorf("rootfs-propagation must be empty or one of private|rprivate|slave|rslave|shared|rshared")
	}
	g.initSpecLinux()
	g.spec.Linux.RootfsPropagation = rp
	return nil
}

// ClearPreStartHooks clear g.spec.Hooks.Prestart.
func (g *Generator) ClearPreStartHooks() {
	if g.spec == nil {
		return
	}
	if g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Prestart = []rspec.Hook{}
}

// AddPreStartHook add a prestart hook into g.spec.Hooks.Prestart.
func (g *Generator) AddPreStartHook(path string, args []string) {
	g.initSpecHooks()
	hook := rspec.Hook{Path: path, Args: args}
	for i, hook := range g.spec.Hooks.Prestart {
		if hook.Path == path {
			g.spec.Hooks.Prestart[i] = hook
			return
		}
	}
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, hook)
}

// AddPreStartHookEnv adds envs of a prestart hook into g.spec.Hooks.Prestart.
func (g *Generator) AddPreStartHookEnv(path string, envs []string) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Prestart {
		if hook.Path == path {
			g.spec.Hooks.Prestart[i].Env = envs
			return
		}
	}
	hook := rspec.Hook{Path: path, Env: envs}
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, hook)
}

// AddPreStartHookTimeout adds timeout of a prestart hook into g.spec.Hooks.Prestart.
func (g *Generator) AddPreStartHookTimeout(path string, timeout int) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Prestart {
		if hook.Path == path {
			g.spec.Hooks.Prestart[i].Timeout = &timeout
			return
		}
	}
	hook := rspec.Hook{Path: path, Timeout: &timeout}
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, hook)
}

// ClearPostStopHooks clear g.spec.Hooks.Poststop.
func (g *Generator) ClearPostStopHooks() {
	if g.spec == nil {
		return
	}
	if g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Poststop = []rspec.Hook{}
}

// AddPostStopHook adds a poststop hook into g.spec.Hooks.Poststop.
func (g *Generator) AddPostStopHook(path string, args []string) {
	g.initSpecHooks()
	hook := rspec.Hook{Path: path, Args: args}
	for i, hook := range g.spec.Hooks.Poststop {
		if hook.Path == path {
			g.spec.Hooks.Poststop[i] = hook
			return
		}
	}
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, hook)
}

// AddPostStopHookEnv adds envs of a poststop hook into g.spec.Hooks.Poststop.
func (g *Generator) AddPostStopHookEnv(path string, envs []string) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststop {
		if hook.Path == path {
			g.spec.Hooks.Poststop[i].Env = envs
			return
		}
	}
	hook := rspec.Hook{Path: path, Env: envs}
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, hook)
}

// AddPostStopHookTimeout adds timeout of a poststop hook into g.spec.Hooks.Poststop.
func (g *Generator) AddPostStopHookTimeout(path string, timeout int) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststop {
		if hook.Path == path {
			g.spec.Hooks.Poststop[i].Timeout = &timeout
			return
		}
	}
	hook := rspec.Hook{Path: path, Timeout: &timeout}
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, hook)
}

// ClearPostStartHooks clear g.spec.Hooks.Poststart.
func (g *Generator) ClearPostStartHooks() {
	if g.spec == nil {
		return
	}
	if g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Poststart = []rspec.Hook{}
}

// AddPostStartHook adds a poststart hook into g.spec.Hooks.Poststart.
func (g *Generator) AddPostStartHook(path string, args []string) {
	g.initSpecHooks()
	hook := rspec.Hook{Path: path, Args: args}
	for i, hook := range g.spec.Hooks.Poststart {
		if hook.Path == path {
			g.spec.Hooks.Poststart[i] = hook
			return
		}
	}
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, hook)
}

// AddPostStartHookEnv adds envs of a poststart hook into g.spec.Hooks.Poststart.
func (g *Generator) AddPostStartHookEnv(path string, envs []string) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststart {
		if hook.Path == path {
			g.spec.Hooks.Poststart[i].Env = envs
			return
		}
	}
	hook := rspec.Hook{Path: path, Env: envs}
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, hook)
}

// AddPostStartHookTimeout adds timeout of a poststart hook into g.spec.Hooks.Poststart.
func (g *Generator) AddPostStartHookTimeout(path string, timeout int) {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststart {
		if hook.Path == path {
			g.spec.Hooks.Poststart[i].Timeout = &timeout
			return
		}
	}
	hook := rspec.Hook{Path: path, Timeout: &timeout}
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, hook)
}

// AddTmpfsMount adds a tmpfs mount into g.spec.Mounts.
func (g *Generator) AddTmpfsMount(dest string, options []string) {
	mnt := rspec.Mount{
		Destination: dest,
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     options,
	}

	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// AddCgroupsMount adds a cgroup mount into g.spec.Mounts.
func (g *Generator) AddCgroupsMount(mountCgroupOption string) error {
	switch mountCgroupOption {
	case "ro":
	case "rw":
	case "no":
		return nil
	default:
		return fmt.Errorf("--mount-cgroups should be one of (ro,rw,no)")
	}

	mnt := rspec.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", mountCgroupOption},
	}
	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)

	return nil
}

// AddBindMount adds a bind mount into g.spec.Mounts.
func (g *Generator) AddBindMount(source, dest string, options []string) {
	if len(options) == 0 {
		options = []string{"rw"}
	}

	// We have to make sure that there is a bind option set, otherwise it won't
	// be an actual bindmount.
	foundBindOption := false
	for _, opt := range options {
		if opt == "bind" || opt == "rbind" {
			foundBindOption = true
			break
		}
	}
	if !foundBindOption {
		options = append(options, "bind")
	}

	mnt := rspec.Mount{
		Destination: dest,
		Type:        "bind",
		Source:      source,
		Options:     options,
	}
	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// SetupPrivileged sets up the privilege-related fields inside g.spec.
func (g *Generator) SetupPrivileged(privileged bool) {
	if privileged { // Add all capabilities in privileged mode.
		var finalCapList []string
		for _, cap := range capability.List() {
			if g.HostSpecific && cap > validate.LastCap() {
				continue
			}
			finalCapList = append(finalCapList, fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())))
		}
		g.initSpecLinux()
		g.initSpecProcessCapabilities()
		g.ClearProcessCapabilities()
		g.spec.Process.Capabilities.Bounding = append(g.spec.Process.Capabilities.Bounding, finalCapList...)
		g.spec.Process.Capabilities.Effective = append(g.spec.Process.Capabilities.Effective, finalCapList...)
		g.spec.Process.Capabilities.Inheritable = append(g.spec.Process.Capabilities.Inheritable, finalCapList...)
		g.spec.Process.Capabilities.Permitted = append(g.spec.Process.Capabilities.Permitted, finalCapList...)
		g.spec.Process.Capabilities.Ambient = append(g.spec.Process.Capabilities.Ambient, finalCapList...)
		g.spec.Process.SelinuxLabel = ""
		g.spec.Process.ApparmorProfile = ""
		g.spec.Linux.Seccomp = nil
	}
}

// ClearProcessCapabilities clear g.spec.Process.Capabilities.
func (g *Generator) ClearProcessCapabilities() {
	if g.spec == nil {
		return
	}
	g.spec.Process.Capabilities.Bounding = []string{}
	g.spec.Process.Capabilities.Effective = []string{}
	g.spec.Process.Capabilities.Inheritable = []string{}
	g.spec.Process.Capabilities.Permitted = []string{}
	g.spec.Process.Capabilities.Ambient = []string{}
}

// AddProcessCapability adds a process capability into g.spec.Process.Capabilities.
func (g *Generator) AddProcessCapability(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

	var foundBounding bool
	for _, cap := range g.spec.Process.Capabilities.Bounding {
		if strings.ToUpper(cap) == cp {
			foundBounding = true
			break
		}
	}
	if !foundBounding {
		g.spec.Process.Capabilities.Bounding = append(g.spec.Process.Capabilities.Bounding, cp)
	}

	var foundEffective bool
	for _, cap := range g.spec.Process.Capabilities.Effective {
		if strings.ToUpper(cap) == cp {
			foundEffective = true
			break
		}
	}
	if !foundEffective {
		g.spec.Process.Capabilities.Effective = append(g.spec.Process.Capabilities.Effective, cp)
	}

	var foundInheritable bool
	for _, cap := range g.spec.Process.Capabilities.Inheritable {
		if strings.ToUpper(cap) == cp {
			foundInheritable = true
			break
		}
	}
	if !foundInheritable {
		g.spec.Process.Capabilities.Inheritable = append(g.spec.Process.Capabilities.Inheritable, cp)
	}

	var foundPermitted bool
	for _, cap := range g.spec.Process.Capabilities.Permitted {
		if strings.ToUpper(cap) == cp {
			foundPermitted = true
			break
		}
	}
	if !foundPermitted {
		g.spec.Process.Capabilities.Permitted = append(g.spec.Process.Capabilities.Permitted, cp)
	}

	var foundAmbient bool
	for _, cap := range g.spec.Process.Capabilities.Ambient {
		if strings.ToUpper(cap) == cp {
			foundAmbient = true
			break
		}
	}
	if !foundAmbient {
		g.spec.Process.Capabilities.Ambient = append(g.spec.Process.Capabilities.Ambient, cp)
	}

	return nil
}

// DropProcessCapability drops a process capability from g.spec.Process.Capabilities.
func (g *Generator) DropProcessCapability(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

	// we don't care about order...and this is way faster...
	removeFunc := func(s []string, i int) []string {
		s[i] = s[len(s)-1]
		return s[:len(s)-1]
	}

	for i, cap := range g.spec.Process.Capabilities.Bounding {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Bounding = removeFunc(g.spec.Process.Capabilities.Bounding, i)
		}
	}

	for i, cap := range g.spec.Process.Capabilities.Effective {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Effective = removeFunc(g.spec.Process.Capabilities.Effective, i)
		}
	}

	for i, cap := range g.spec.Process.Capabilities.Inheritable {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Inheritable = removeFunc(g.spec.Process.Capabilities.Inheritable, i)
		}
	}

	for i, cap := range g.spec.Process.Capabilities.Permitted {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Permitted = removeFunc(g.spec.Process.Capabilities.Permitted, i)
		}
	}

	for i, cap := range g.spec.Process.Capabilities.Ambient {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Ambient = removeFunc(g.spec.Process.Capabilities.Ambient, i)
		}
	}

	return nil
}

func mapStrToNamespace(ns string, path string) (rspec.LinuxNamespace, error) {
	switch ns {
	case "network":
		return rspec.LinuxNamespace{Type: rspec.NetworkNamespace, Path: path}, nil
	case "pid":
		return rspec.LinuxNamespace{Type: rspec.PIDNamespace, Path: path}, nil
	case "mount":
		return rspec.LinuxNamespace{Type: rspec.MountNamespace, Path: path}, nil
	case "ipc":
		return rspec.LinuxNamespace{Type: rspec.IPCNamespace, Path: path}, nil
	case "uts":
		return rspec.LinuxNamespace{Type: rspec.UTSNamespace, Path: path}, nil
	case "user":
		return rspec.LinuxNamespace{Type: rspec.UserNamespace, Path: path}, nil
	case "cgroup":
		return rspec.LinuxNamespace{Type: rspec.CgroupNamespace, Path: path}, nil
	default:
		return rspec.LinuxNamespace{}, fmt.Errorf("unrecognized namespace %q", ns)
	}
}

// ClearLinuxNamespaces clear g.spec.Linux.Namespaces.
func (g *Generator) ClearLinuxNamespaces() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.Namespaces = []rspec.LinuxNamespace{}
}

// AddOrReplaceLinuxNamespace adds or replaces a namespace inside
// g.spec.Linux.Namespaces.
func (g *Generator) AddOrReplaceLinuxNamespace(ns string, path string) error {
	namespace, err := mapStrToNamespace(ns, path)
	if err != nil {
		return err
	}

	g.initSpecLinux()
	for i, ns := range g.spec.Linux.Namespaces {
		if ns.Type == namespace.Type {
			g.spec.Linux.Namespaces[i] = namespace
			return nil
		}
	}
	g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces, namespace)
	return nil
}

// RemoveLinuxNamespace removes a namespace from g.spec.Linux.Namespaces.
func (g *Generator) RemoveLinuxNamespace(ns string) error {
	namespace, err := mapStrToNamespace(ns, "")
	if err != nil {
		return err
	}

	if g.spec == nil || g.spec.Linux == nil {
		return nil
	}
	for i, ns := range g.spec.Linux.Namespaces {
		if ns.Type == namespace.Type {
			g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces[:i], g.spec.Linux.Namespaces[i+1:]...)
			return nil
		}
	}
	return nil
}

// AddDevice - add a device into g.spec.Linux.Devices
func (g *Generator) AddDevice(device rspec.LinuxDevice) {
	g.initSpecLinux()

	for i, dev := range g.spec.Linux.Devices {
		if dev.Path == device.Path {
			g.spec.Linux.Devices[i] = device
			return
		}
		if dev.Type == device.Type && dev.Major == device.Major && dev.Minor == device.Minor {
			fmt.Fprintln(os.Stderr, "WARNING: The same type, major and minor should not be used for multiple devices.")
		}
	}

	g.spec.Linux.Devices = append(g.spec.Linux.Devices, device)
}

// RemoveDevice remove a device from g.spec.Linux.Devices
func (g *Generator) RemoveDevice(path string) error {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Devices == nil {
		return nil
	}

	for i, device := range g.spec.Linux.Devices {
		if device.Path == path {
			g.spec.Linux.Devices = append(g.spec.Linux.Devices[:i], g.spec.Linux.Devices[i+1:]...)
			return nil
		}
	}
	return nil
}

// ClearLinuxDevices clears g.spec.Linux.Devices
func (g *Generator) ClearLinuxDevices() {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Devices == nil {
		return
	}

	g.spec.Linux.Devices = []rspec.LinuxDevice{}
}

// strPtr returns the pointer pointing to the string s.
func strPtr(s string) *string { return &s }

// SetSyscallAction adds rules for syscalls with the specified action
func (g *Generator) SetSyscallAction(arguments seccomp.SyscallOpts) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseSyscallFlag(arguments, g.spec.Linux.Seccomp)
}

// SetDefaultSeccompAction sets the default action for all syscalls not defined
// and then removes any syscall rules with this action already specified.
func (g *Generator) SetDefaultSeccompAction(action string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseDefaultAction(action, g.spec.Linux.Seccomp)
}

// SetDefaultSeccompActionForce only sets the default action for all syscalls not defined
func (g *Generator) SetDefaultSeccompActionForce(action string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseDefaultActionForce(action, g.spec.Linux.Seccomp)
}

// SetSeccompArchitecture sets the supported seccomp architectures
func (g *Generator) SetSeccompArchitecture(architecture string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseArchitectureFlag(architecture, g.spec.Linux.Seccomp)
}

// RemoveSeccompRule removes rules for any specified syscalls
func (g *Generator) RemoveSeccompRule(arguments string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.RemoveAction(arguments, g.spec.Linux.Seccomp)
}

// RemoveAllSeccompRules removes all syscall rules
func (g *Generator) RemoveAllSeccompRules() error {
	g.initSpecLinuxSeccomp()
	return seccomp.RemoveAllSeccompRules(g.spec.Linux.Seccomp)
}

// AddLinuxMaskedPaths adds masked paths into g.spec.Linux.MaskedPaths.
func (g *Generator) AddLinuxMaskedPaths(path string) {
	g.initSpecLinux()
	g.spec.Linux.MaskedPaths = append(g.spec.Linux.MaskedPaths, path)
}

// AddLinuxReadonlyPaths adds readonly paths into g.spec.Linux.MaskedPaths.
func (g *Generator) AddLinuxReadonlyPaths(path string) {
	g.initSpecLinux()
	g.spec.Linux.ReadonlyPaths = append(g.spec.Linux.ReadonlyPaths, path)
}
