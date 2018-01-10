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

	// we don't care about order...and this is way faster...
	removeFunc = func(s []string, i int) []string {
		s[i] = s[len(s)-1]
		return s[:len(s)-1]
	}
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
			Path:     "rootfs",
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
		return Generator{}, err
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
	if g.spec == nil || g.spec.Process == nil {
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
func (g *Generator) RemoveProcessRlimits(rType string) {
	if g.spec == nil || g.spec.Process == nil {
		return
	}
	for i, rlimit := range g.spec.Process.Rlimits {
		if rlimit.Type == rType {
			g.spec.Process.Rlimits = append(g.spec.Process.Rlimits[:i], g.spec.Process.Rlimits[i+1:]...)
			return
		}
	}
}

// ClearProcessRlimits clear g.spec.Process.Rlimits.
func (g *Generator) ClearProcessRlimits() {
	if g.spec == nil || g.spec.Process == nil {
		return
	}
	g.spec.Process.Rlimits = []rspec.POSIXRlimit{}
}

// ClearProcessAdditionalGids clear g.spec.Process.AdditionalGids.
func (g *Generator) ClearProcessAdditionalGids() {
	if g.spec == nil || g.spec.Process == nil {
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

// SetLinuxIntelRdtL3CacheSchema sets g.spec.Linux.IntelRdt.L3CacheSchema
func (g *Generator) SetLinuxIntelRdtL3CacheSchema(schema string) {
	g.initSpecLinuxIntelRdt()
	g.spec.Linux.IntelRdt.L3CacheSchema = schema
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

// SetLinuxResourcesBlockIOLeafWeight sets g.spec.Linux.Resources.BlockIO.LeafWeight.
func (g *Generator) SetLinuxResourcesBlockIOLeafWeight(weight uint16) {
	g.initSpecLinuxResourcesBlockIO()
	g.spec.Linux.Resources.BlockIO.LeafWeight = &weight
}

// AddLinuxResourcesBlockIOLeafWeightDevice adds or sets g.spec.Linux.Resources.BlockIO.WeightDevice.LeafWeight.
func (g *Generator) AddLinuxResourcesBlockIOLeafWeightDevice(major int64, minor int64, weight uint16) {
	g.initSpecLinuxResourcesBlockIO()
	for i, weightDevice := range g.spec.Linux.Resources.BlockIO.WeightDevice {
		if weightDevice.Major == major && weightDevice.Minor == minor {
			g.spec.Linux.Resources.BlockIO.WeightDevice[i].LeafWeight = &weight
			return
		}
	}
	weightDevice := new(rspec.LinuxWeightDevice)
	weightDevice.Major = major
	weightDevice.Minor = minor
	weightDevice.LeafWeight = &weight
	g.spec.Linux.Resources.BlockIO.WeightDevice = append(g.spec.Linux.Resources.BlockIO.WeightDevice, *weightDevice)
}

// DropLinuxResourcesBlockIOLeafWeightDevice drops a item form g.spec.Linux.Resources.BlockIO.WeightDevice.LeafWeight
func (g *Generator) DropLinuxResourcesBlockIOLeafWeightDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	for i, weightDevice := range g.spec.Linux.Resources.BlockIO.WeightDevice {
		if weightDevice.Major == major && weightDevice.Minor == minor {
			if weightDevice.Weight != nil {
				newWeightDevice := new(rspec.LinuxWeightDevice)
				newWeightDevice.Major = major
				newWeightDevice.Minor = minor
				newWeightDevice.Weight = weightDevice.Weight
				g.spec.Linux.Resources.BlockIO.WeightDevice[i] = *newWeightDevice
			} else {
				g.spec.Linux.Resources.BlockIO.WeightDevice = append(g.spec.Linux.Resources.BlockIO.WeightDevice[:i], g.spec.Linux.Resources.BlockIO.WeightDevice[i+1:]...)
			}
			return
		}
	}
}

// SetLinuxResourcesBlockIOWeight sets g.spec.Linux.Resources.BlockIO.Weight.
func (g *Generator) SetLinuxResourcesBlockIOWeight(weight uint16) {
	g.initSpecLinuxResourcesBlockIO()
	g.spec.Linux.Resources.BlockIO.Weight = &weight
}

// AddLinuxResourcesBlockIOWeightDevice adds or sets g.spec.Linux.Resources.BlockIO.WeightDevice.Weight.
func (g *Generator) AddLinuxResourcesBlockIOWeightDevice(major int64, minor int64, weight uint16) {
	g.initSpecLinuxResourcesBlockIO()
	for i, weightDevice := range g.spec.Linux.Resources.BlockIO.WeightDevice {
		if weightDevice.Major == major && weightDevice.Minor == minor {
			g.spec.Linux.Resources.BlockIO.WeightDevice[i].Weight = &weight
			return
		}
	}
	weightDevice := new(rspec.LinuxWeightDevice)
	weightDevice.Major = major
	weightDevice.Minor = minor
	weightDevice.Weight = &weight
	g.spec.Linux.Resources.BlockIO.WeightDevice = append(g.spec.Linux.Resources.BlockIO.WeightDevice, *weightDevice)
}

// DropLinuxResourcesBlockIOWeightDevice drops a item form g.spec.Linux.Resources.BlockIO.WeightDevice.Weight
func (g *Generator) DropLinuxResourcesBlockIOWeightDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	for i, weightDevice := range g.spec.Linux.Resources.BlockIO.WeightDevice {
		if weightDevice.Major == major && weightDevice.Minor == minor {
			if weightDevice.LeafWeight != nil {
				newWeightDevice := new(rspec.LinuxWeightDevice)
				newWeightDevice.Major = major
				newWeightDevice.Minor = minor
				newWeightDevice.LeafWeight = weightDevice.LeafWeight
				g.spec.Linux.Resources.BlockIO.WeightDevice[i] = *newWeightDevice
			} else {
				g.spec.Linux.Resources.BlockIO.WeightDevice = append(g.spec.Linux.Resources.BlockIO.WeightDevice[:i], g.spec.Linux.Resources.BlockIO.WeightDevice[i+1:]...)
			}
			return
		}
	}
}

// AddLinuxResourcesBlockIOThrottleReadBpsDevice adds or sets g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice.
func (g *Generator) AddLinuxResourcesBlockIOThrottleReadBpsDevice(major int64, minor int64, rate uint64) {
	g.initSpecLinuxResourcesBlockIO()
	throttleDevices := addOrReplaceBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice, major, minor, rate)
	g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice = throttleDevices
}

// DropLinuxResourcesBlockIOThrottleReadBpsDevice drops a item from g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice.
func (g *Generator) DropLinuxResourcesBlockIOThrottleReadBpsDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	throttleDevices := dropBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice, major, minor)
	g.spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice = throttleDevices
}

// AddLinuxResourcesBlockIOThrottleReadIOPSDevice adds or sets g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice.
func (g *Generator) AddLinuxResourcesBlockIOThrottleReadIOPSDevice(major int64, minor int64, rate uint64) {
	g.initSpecLinuxResourcesBlockIO()
	throttleDevices := addOrReplaceBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice, major, minor, rate)
	g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice = throttleDevices
}

// DropLinuxResourcesBlockIOThrottleReadIOPSDevice drops a item from g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice.
func (g *Generator) DropLinuxResourcesBlockIOThrottleReadIOPSDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	throttleDevices := dropBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice, major, minor)
	g.spec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice = throttleDevices
}

// AddLinuxResourcesBlockIOThrottleWriteBpsDevice adds or sets g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice.
func (g *Generator) AddLinuxResourcesBlockIOThrottleWriteBpsDevice(major int64, minor int64, rate uint64) {
	g.initSpecLinuxResourcesBlockIO()
	throttleDevices := addOrReplaceBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice, major, minor, rate)
	g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice = throttleDevices
}

// DropLinuxResourcesBlockIOThrottleWriteBpsDevice drops a item from g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice.
func (g *Generator) DropLinuxResourcesBlockIOThrottleWriteBpsDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	throttleDevices := dropBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice, major, minor)
	g.spec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice = throttleDevices
}

// AddLinuxResourcesBlockIOThrottleWriteIOPSDevice adds or sets g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice.
func (g *Generator) AddLinuxResourcesBlockIOThrottleWriteIOPSDevice(major int64, minor int64, rate uint64) {
	g.initSpecLinuxResourcesBlockIO()
	throttleDevices := addOrReplaceBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice, major, minor, rate)
	g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice = throttleDevices
}

// DropLinuxResourcesBlockIOThrottleWriteIOPSDevice drops a item from g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice.
func (g *Generator) DropLinuxResourcesBlockIOThrottleWriteIOPSDevice(major int64, minor int64) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.BlockIO == nil {
		return
	}

	throttleDevices := dropBlockIOThrottleDevice(g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice, major, minor)
	g.spec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice = throttleDevices
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
func (g *Generator) DropLinuxResourcesHugepageLimit(pageSize string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil {
		return
	}

	for i, pageLimit := range g.spec.Linux.Resources.HugepageLimits {
		if pageLimit.Pagesize == pageSize {
			g.spec.Linux.Resources.HugepageLimits = append(g.spec.Linux.Resources.HugepageLimits[:i], g.spec.Linux.Resources.HugepageLimits[i+1:]...)
			return
		}
	}
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
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil || g.spec.Linux.Resources.Network == nil {
		return
	}

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
	case "unbindable":
	case "runbindable":
	default:
		return fmt.Errorf("rootfs-propagation %q must be empty or one of (r)private|(r)slave|(r)shared|(r)unbindable", rp)
	}
	g.initSpecLinux()
	g.spec.Linux.RootfsPropagation = rp
	return nil
}

// ClearPreStartHooks clear g.spec.Hooks.Prestart.
func (g *Generator) ClearPreStartHooks() {
	if g.spec == nil || g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Prestart = []rspec.Hook{}
}

// AddPreStartHook add a prestart hook into g.spec.Hooks.Prestart.
func (g *Generator) AddPreStartHook(preStartHook rspec.Hook) error {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Prestart {
		if hook.Path == preStartHook.Path {
			g.spec.Hooks.Prestart[i] = preStartHook
			return nil
		}
	}
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, preStartHook)
	return nil
}

// ClearPostStopHooks clear g.spec.Hooks.Poststop.
func (g *Generator) ClearPostStopHooks() {
	if g.spec == nil || g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Poststop = []rspec.Hook{}
}

// AddPostStopHook adds a poststop hook into g.spec.Hooks.Poststop.
func (g *Generator) AddPostStopHook(postStopHook rspec.Hook) error {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststop {
		if hook.Path == postStopHook.Path {
			g.spec.Hooks.Poststop[i] = postStopHook
			return nil
		}
	}
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, postStopHook)
	return nil
}

// ClearPostStartHooks clear g.spec.Hooks.Poststart.
func (g *Generator) ClearPostStartHooks() {
	if g.spec == nil || g.spec.Hooks == nil {
		return
	}
	g.spec.Hooks.Poststart = []rspec.Hook{}
}

// AddPostStartHook adds a poststart hook into g.spec.Hooks.Poststart.
func (g *Generator) AddPostStartHook(postStartHook rspec.Hook) error {
	g.initSpecHooks()
	for i, hook := range g.spec.Hooks.Poststart {
		if hook.Path == postStartHook.Path {
			g.spec.Hooks.Poststart[i] = postStartHook
			return nil
		}
	}
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, postStartHook)
	return nil
}

// AddMount adds a mount into g.spec.Mounts.
func (g *Generator) AddMount(mnt rspec.Mount) {
	g.initSpec()

	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// RemoveMount removes a mount point on the dest directory
func (g *Generator) RemoveMount(dest string) {
	g.initSpec()

	for index, mount := range g.spec.Mounts {
		if mount.Destination == dest {
			g.spec.Mounts = append(g.spec.Mounts[:index], g.spec.Mounts[index+1:]...)
			return
		}
	}
}

// Mounts returns the list of mounts
func (g *Generator) Mounts() []rspec.Mount {
	g.initSpec()

	return g.spec.Mounts
}

// ClearMounts clear g.spec.Mounts
func (g *Generator) ClearMounts() {
	if g.spec == nil {
		return
	}
	g.spec.Mounts = []rspec.Mount{}
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
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return
	}
	g.spec.Process.Capabilities.Bounding = []string{}
	g.spec.Process.Capabilities.Effective = []string{}
	g.spec.Process.Capabilities.Inheritable = []string{}
	g.spec.Process.Capabilities.Permitted = []string{}
	g.spec.Process.Capabilities.Ambient = []string{}
}

// AddProcessCapabilityAmbient adds a process capability into g.spec.Process.Capabilities.Ambient.
func (g *Generator) AddProcessCapabilityAmbient(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

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

// AddProcessCapabilityBounding adds a process capability into g.spec.Process.Capabilities.Bounding.
func (g *Generator) AddProcessCapabilityBounding(c string) error {
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

	return nil
}

// AddProcessCapabilityEffective adds a process capability into g.spec.Process.Capabilities.Effective.
func (g *Generator) AddProcessCapabilityEffective(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

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

	return nil
}

// AddProcessCapabilityInheritable adds a process capability into g.spec.Process.Capabilities.Inheritable.
func (g *Generator) AddProcessCapabilityInheritable(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

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

	return nil
}

// AddProcessCapabilityPermitted adds a process capability into g.spec.Process.Capabilities.Permitted.
func (g *Generator) AddProcessCapabilityPermitted(c string) error {
	cp := strings.ToUpper(c)
	if err := validate.CapValid(cp, g.HostSpecific); err != nil {
		return err
	}

	g.initSpecProcessCapabilities()

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

	return nil
}

// DropProcessCapabilityAmbient drops a process capability from g.spec.Process.Capabilities.Ambient.
func (g *Generator) DropProcessCapabilityAmbient(c string) error {
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return nil
	}

	cp := strings.ToUpper(c)
	for i, cap := range g.spec.Process.Capabilities.Ambient {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Ambient = removeFunc(g.spec.Process.Capabilities.Ambient, i)
		}
	}

	return validate.CapValid(cp, false)
}

// DropProcessCapabilityBounding drops a process capability from g.spec.Process.Capabilities.Bounding.
func (g *Generator) DropProcessCapabilityBounding(c string) error {
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return nil
	}

	cp := strings.ToUpper(c)
	for i, cap := range g.spec.Process.Capabilities.Bounding {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Bounding = removeFunc(g.spec.Process.Capabilities.Bounding, i)
		}
	}

	return validate.CapValid(cp, false)
}

// DropProcessCapabilityEffective drops a process capability from g.spec.Process.Capabilities.Effective.
func (g *Generator) DropProcessCapabilityEffective(c string) error {
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return nil
	}

	cp := strings.ToUpper(c)
	for i, cap := range g.spec.Process.Capabilities.Effective {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Effective = removeFunc(g.spec.Process.Capabilities.Effective, i)
		}
	}

	return validate.CapValid(cp, false)
}

// DropProcessCapabilityInheritable drops a process capability from g.spec.Process.Capabilities.Inheritable.
func (g *Generator) DropProcessCapabilityInheritable(c string) error {
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return nil
	}

	cp := strings.ToUpper(c)
	for i, cap := range g.spec.Process.Capabilities.Inheritable {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Inheritable = removeFunc(g.spec.Process.Capabilities.Inheritable, i)
		}
	}

	return validate.CapValid(cp, false)
}

// DropProcessCapabilityPermitted drops a process capability from g.spec.Process.Capabilities.Permitted.
func (g *Generator) DropProcessCapabilityPermitted(c string) error {
	if g.spec == nil || g.spec.Process == nil || g.spec.Process.Capabilities == nil {
		return nil
	}

	cp := strings.ToUpper(c)
	for i, cap := range g.spec.Process.Capabilities.Permitted {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities.Ambient = removeFunc(g.spec.Process.Capabilities.Ambient, i)
		}
	}

	return validate.CapValid(cp, false)
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
func (g *Generator) RemoveDevice(path string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Devices == nil {
		return
	}

	for i, device := range g.spec.Linux.Devices {
		if device.Path == path {
			g.spec.Linux.Devices = append(g.spec.Linux.Devices[:i], g.spec.Linux.Devices[i+1:]...)
			return
		}
	}
}

// ClearLinuxDevices clears g.spec.Linux.Devices
func (g *Generator) ClearLinuxDevices() {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Devices == nil {
		return
	}

	g.spec.Linux.Devices = []rspec.LinuxDevice{}
}

// AddLinuxResourcesDevice - add a device into g.spec.Linux.Resources.Devices
func (g *Generator) AddLinuxResourcesDevice(allow bool, devType string, major, minor *int64, access string) {
	g.initSpecLinuxResources()

	device := rspec.LinuxDeviceCgroup{
		Allow:  allow,
		Type:   devType,
		Access: access,
		Major:  major,
		Minor:  minor,
	}
	g.spec.Linux.Resources.Devices = append(g.spec.Linux.Resources.Devices, device)
}

// RemoveLinuxResourcesDevice - remove a device from g.spec.Linux.Resources.Devices
func (g *Generator) RemoveLinuxResourcesDevice(allow bool, devType string, major, minor *int64, access string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Resources == nil {
		return
	}
	for i, device := range g.spec.Linux.Resources.Devices {
		if device.Allow == allow &&
			(devType == device.Type || (devType != "" && device.Type != "" && devType == device.Type)) &&
			(access == device.Access || (access != "" && device.Access != "" && access == device.Access)) &&
			(major == device.Major || (major != nil && device.Major != nil && *major == *device.Major)) &&
			(minor == device.Minor || (minor != nil && device.Minor != nil && *minor == *device.Minor)) {

			g.spec.Linux.Resources.Devices = append(g.spec.Linux.Resources.Devices[:i], g.spec.Linux.Resources.Devices[i+1:]...)
			return
		}
	}
	return
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

func addOrReplaceBlockIOThrottleDevice(tmpList []rspec.LinuxThrottleDevice, major int64, minor int64, rate uint64) []rspec.LinuxThrottleDevice {
	throttleDevices := tmpList
	for i, throttleDevice := range throttleDevices {
		if throttleDevice.Major == major && throttleDevice.Minor == minor {
			throttleDevices[i].Rate = rate
			return throttleDevices
		}
	}
	throttleDevice := new(rspec.LinuxThrottleDevice)
	throttleDevice.Major = major
	throttleDevice.Minor = minor
	throttleDevice.Rate = rate
	throttleDevices = append(throttleDevices, *throttleDevice)

	return throttleDevices
}

func dropBlockIOThrottleDevice(tmpList []rspec.LinuxThrottleDevice, major int64, minor int64) []rspec.LinuxThrottleDevice {
	throttleDevices := tmpList
	for i, throttleDevice := range throttleDevices {
		if throttleDevice.Major == major && throttleDevice.Minor == minor {
			throttleDevices = append(throttleDevices[:i], throttleDevices[i+1:]...)
			return throttleDevices
		}
	}

	return throttleDevices
}

// AddSolarisAnet adds network into g.spec.Solaris.Anet
func (g *Generator) AddSolarisAnet(anet rspec.SolarisAnet) {
	g.initSpecSolaris()
	g.spec.Solaris.Anet = append(g.spec.Solaris.Anet, anet)
}

// SetSolarisCappedCPUNcpus sets g.spec.Solaris.CappedCPU.Ncpus
func (g *Generator) SetSolarisCappedCPUNcpus(ncpus string) {
	g.initSpecSolarisCappedCPU()
	g.spec.Solaris.CappedCPU.Ncpus = ncpus
}

// SetSolarisCappedMemoryPhysical sets g.spec.Solaris.CappedMemory.Physical
func (g *Generator) SetSolarisCappedMemoryPhysical(physical string) {
	g.initSpecSolarisCappedMemory()
	g.spec.Solaris.CappedMemory.Physical = physical
}

// SetSolarisCappedMemorySwap sets g.spec.Solaris.CappedMemory.Swap
func (g *Generator) SetSolarisCappedMemorySwap(swap string) {
	g.initSpecSolarisCappedMemory()
	g.spec.Solaris.CappedMemory.Swap = swap
}

// SetSolarisLimitPriv sets g.spec.Solaris.LimitPriv
func (g *Generator) SetSolarisLimitPriv(limitPriv string) {
	g.initSpecSolaris()
	g.spec.Solaris.LimitPriv = limitPriv
}

// SetSolarisMaxShmMemory sets g.spec.Solaris.MaxShmMemory
func (g *Generator) SetSolarisMaxShmMemory(memory string) {
	g.initSpecSolaris()
	g.spec.Solaris.MaxShmMemory = memory
}

// SetSolarisMilestone sets g.spec.Solaris.Milestone
func (g *Generator) SetSolarisMilestone(milestone string) {
	g.initSpecSolaris()
	g.spec.Solaris.Milestone = milestone
}

// SetWindowsHypervUntilityVMPath sets g.spec.Windows.HyperV.UtilityVMPath.
func (g *Generator) SetWindowsHypervUntilityVMPath(path string) {
	g.initSpecWindowsHyperV()
	g.spec.Windows.HyperV.UtilityVMPath = path
}

// SetWinodwsIgnoreFlushesDuringBoot sets g.spec.Winodws.IgnoreFlushesDuringBoot.
func (g *Generator) SetWinodwsIgnoreFlushesDuringBoot(ignore bool) {
	g.initSpecWindows()
	g.spec.Windows.IgnoreFlushesDuringBoot = ignore
}

// AddWindowsLayerFolders adds layer folders into  g.spec.Windows.LayerFolders.
func (g *Generator) AddWindowsLayerFolders(folder string) {
	g.initSpecWindows()
	g.spec.Windows.LayerFolders = append(g.spec.Windows.LayerFolders, folder)
}

// SetWindowsNetwork sets g.spec.Windows.Network.
func (g *Generator) SetWindowsNetwork(network rspec.WindowsNetwork) {
	g.initSpecWindows()
	g.spec.Windows.Network = &network
}

// SetWindowsResourcesCPU sets g.spec.Windows.Resources.CPU.
func (g *Generator) SetWindowsResourcesCPU(cpu rspec.WindowsCPUResources) {
	g.initSpecWindowsResources()
	g.spec.Windows.Resources.CPU = &cpu
}

// SetWindowsResourcesMemoryLimit sets g.spec.Windows.Resources.Memory.Limit.
func (g *Generator) SetWindowsResourcesMemoryLimit(limit uint64) {
	g.initSpecWindowsResourcesMemory()
	g.spec.Windows.Resources.Memory.Limit = &limit
}

// SetWindowsResourcesStorage sets g.spec.Windows.Resources.Storage.
func (g *Generator) SetWindowsResourcesStorage(storage rspec.WindowsStorageResources) {
	g.initSpecWindowsResources()
	g.spec.Windows.Resources.Storage = &storage
}

// SetWinodwsServicing sets g.spec.Winodws.Servicing.
func (g *Generator) SetWinodwsServicing(servicing bool) {
	g.initSpecWindows()
	g.spec.Windows.Servicing = servicing
}
