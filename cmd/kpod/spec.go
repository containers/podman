package main

import (
	"fmt"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	ann "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

// Parses information needed to create a container into an OCI runtime spec
func createConfigToOCISpec(config *createConfig) (*spec.Spec, error) {
	spec := config.GetDefaultLinuxSpec()
	spec.Process.Cwd = config.workDir
	spec.Process.Args = config.command

	spec.Process.Terminal = config.tty

	// User and Group must go together
	spec.Process.User.UID = config.user
	spec.Process.User.GID = config.group
	spec.Process.User.AdditionalGids = config.groupAdd

	spec.Process.Env = config.env

	//TODO
	// Need examples of capacity additions so I can load that properly

	spec.Root.Readonly = config.readOnlyRootfs
	spec.Hostname = config.hostname

	// BIND MOUNTS
	spec.Mounts = append(spec.Mounts, config.GetVolumeMounts()...)

	// TMPFS MOUNTS
	spec.Mounts = append(spec.Mounts, config.GetTmpfsMounts()...)

	// RESOURCES - MEMORY
	spec.Linux.Sysctl = config.sysctl

	if config.resources.memory != 0 {
		spec.Linux.Resources.Memory.Limit = &config.resources.memory
	}
	if config.resources.memoryReservation != 0 {
		spec.Linux.Resources.Memory.Reservation = &config.resources.memoryReservation
	}
	if config.resources.memorySwap != 0 {
		spec.Linux.Resources.Memory.Swap = &config.resources.memorySwap
	}
	if config.resources.kernelMemory != 0 {
		spec.Linux.Resources.Memory.Kernel = &config.resources.kernelMemory
	}
	if config.resources.memorySwapiness != 0 {
		spec.Linux.Resources.Memory.Swappiness = &config.resources.memorySwapiness
	}
	if config.resources.disableOomKiller {
		spec.Linux.Resources.Memory.DisableOOMKiller = &config.resources.disableOomKiller
	}

	// RESOURCES - CPU

	if config.resources.cpuShares != 0 {
		spec.Linux.Resources.CPU.Shares = &config.resources.cpuShares
	}
	if config.resources.cpuQuota != 0 {
		spec.Linux.Resources.CPU.Quota = &config.resources.cpuQuota
	}
	if config.resources.cpuPeriod != 0 {
		spec.Linux.Resources.CPU.Period = &config.resources.cpuPeriod
	}
	if config.resources.cpuRtRuntime != 0 {
		spec.Linux.Resources.CPU.RealtimeRuntime = &config.resources.cpuRtRuntime
	}
	if config.resources.cpuRtPeriod != 0 {
		spec.Linux.Resources.CPU.RealtimePeriod = &config.resources.cpuRtPeriod
	}
	if config.resources.cpus != "" {
		spec.Linux.Resources.CPU.Cpus = config.resources.cpus
	}
	if config.resources.cpusetMems != "" {
		spec.Linux.Resources.CPU.Mems = config.resources.cpusetMems
	}

	// RESOURCES - PIDS
	if config.resources.pidsLimit != 0 {
		spec.Linux.Resources.Pids.Limit = config.resources.pidsLimit
	}

	/*
				Capabilities: &spec.LinuxCapabilities{
				// Rlimits []PosixRlimit // Where does this come from
				// Type string
				// Hard uint64
				// Limit uint64
				// NoNewPrivileges bool // No user input for this
				// ApparmorProfile string // No user input for this
				OOMScoreAdj: &config.resources.oomScoreAdj,
				// Selinuxlabel
			},
			Hooks: &spec.Hooks{},
			//Annotations
				Resources: &spec.LinuxResources{
					Devices: config.GetDefaultDevices(),
					BlockIO: &blkio,
					//HugepageLimits:
					Network: &spec.LinuxNetwork{
					// ClassID *uint32
					// Priorites []LinuxInterfacePriority
					},
				},
				//CgroupsPath:
				//Namespaces: []LinuxNamespace
				//Devices
				Seccomp: &spec.LinuxSeccomp{
				// DefaultAction:
				// Architectures
				// Syscalls:
				},
				// RootfsPropagation
				// MaskedPaths
				// ReadonlyPaths:
				// MountLabel
				// IntelRdt
			},
		}
	*/
	return &spec, nil
}

func (c *createConfig) CreateBlockIO() (spec.LinuxBlockIO, error) {
	bio := spec.LinuxBlockIO{}
	bio.Weight = &c.resources.blkioWeight
	if len(c.resources.blkioDevice) > 0 {
		var lwds []spec.LinuxWeightDevice
		for _, i := range c.resources.blkioDevice {
			wd, err := validateweightDevice(i)
			if err != nil {
				return bio, errors.Wrapf(err, "invalid values for blkio-weight-device")
			}
			wdStat := getStatFromPath(wd.path)
			lwd := spec.LinuxWeightDevice{
				Weight: &wd.weight,
			}
			lwd.Major = int64(unix.Major(wdStat.Rdev))
			lwd.Minor = int64(unix.Minor(wdStat.Rdev))
			lwds = append(lwds, lwd)
		}
	}
	if len(c.resources.deviceReadBps) > 0 {
		readBps, err := makeThrottleArray(c.resources.deviceReadBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadBpsDevice = readBps
	}
	if len(c.resources.deviceWriteBps) > 0 {
		writeBpds, err := makeThrottleArray(c.resources.deviceWriteBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteBpsDevice = writeBpds
	}
	if len(c.resources.deviceReadIops) > 0 {
		readIops, err := makeThrottleArray(c.resources.deviceReadIops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadIOPSDevice = readIops
	}
	if len(c.resources.deviceWriteIops) > 0 {
		writeIops, err := makeThrottleArray(c.resources.deviceWriteIops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteIOPSDevice = writeIops
	}

	return bio, nil
}

func (c *createConfig) GetDefaultMounts() []spec.Mount {
	// Default to 64K default per man page
	shmSize := "65536k"
	if c.resources.shmSize != "" {
		shmSize = c.resources.shmSize
	}
	return []spec.Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options:     []string{"nosuid", "noexec", "nodev"},
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
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "ro"},
		},
		{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"ro", "nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", fmt.Sprintf("size=%s", shmSize)},
		},
	}
}

func iPtr(i int64) *int64 { return &i }

func (c *createConfig) GetDefaultDevices() []spec.LinuxDeviceCgroup {
	return []spec.LinuxDeviceCgroup{
		{
			Allow:  false,
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(1),
			Minor:  iPtr(5),
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(1),
			Minor:  iPtr(3),
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(1),
			Minor:  iPtr(9),
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(1),
			Minor:  iPtr(8),
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(5),
			Minor:  iPtr(0),
			Access: "rwm",
		},
		{
			Allow:  true,
			Type:   "c",
			Major:  iPtr(5),
			Minor:  iPtr(1),
			Access: "rwm",
		},
		{
			Allow:  false,
			Type:   "c",
			Major:  iPtr(10),
			Minor:  iPtr(229),
			Access: "rwm",
		},
	}
}

func defaultCapabilities() []string {
	return []string{
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
	}
}

func (c *createConfig) GetDefaultLinuxSpec() spec.Spec {
	s := spec.Spec{
		Version: spec.Version,
		Root:    &spec.Root{},
	}
	s.Annotations = c.GetAnnotations()
	s.Mounts = c.GetDefaultMounts()
	s.Process = &spec.Process{
		Capabilities: &spec.LinuxCapabilities{
			Bounding:    defaultCapabilities(),
			Permitted:   defaultCapabilities(),
			Inheritable: defaultCapabilities(),
			Effective:   defaultCapabilities(),
		},
	}
	s.Linux = &spec.Linux{
		MaskedPaths: []string{
			"/proc/kcore",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
		},
		ReadonlyPaths: []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		},
		Namespaces: []spec.LinuxNamespace{
			{Type: "mount"},
			{Type: "network"},
			{Type: "uts"},
			{Type: "pid"},
			{Type: "ipc"},
		},
		Devices: []spec.LinuxDevice{},
		Resources: &spec.LinuxResources{
			Devices: c.GetDefaultDevices(),
		},
	}

	return s
}

// GetAnnotations returns the all the annotations for the container
func (c *createConfig) GetAnnotations() map[string]string {
	a := getDefaultAnnotations()
	// TODO
	// Which annotations do we want added by default
	if c.tty {
		a["io.kubernetes.cri-o.TTY"] = "true"
	}
	return a
}

func getDefaultAnnotations() map[string]string {
	var annotations map[string]string
	annotations = make(map[string]string)
	annotations[ann.Annotations] = ""
	annotations[ann.ContainerID] = ""
	annotations[ann.ContainerName] = ""
	annotations[ann.ContainerType] = ""
	annotations[ann.Created] = ""
	annotations[ann.HostName] = ""
	annotations[ann.IP] = ""
	annotations[ann.Image] = ""
	annotations[ann.ImageName] = ""
	annotations[ann.ImageRef] = ""
	annotations[ann.KubeName] = ""
	annotations[ann.Labels] = ""
	annotations[ann.LogPath] = ""
	annotations[ann.Metadata] = ""
	annotations[ann.Name] = ""
	annotations[ann.PrivilegedRuntime] = ""
	annotations[ann.ResolvPath] = ""
	annotations[ann.HostnamePath] = ""
	annotations[ann.SandboxID] = ""
	annotations[ann.SandboxName] = ""
	annotations[ann.ShmPath] = ""
	annotations[ann.MountPoint] = ""
	annotations[ann.TrustedSandbox] = ""
	annotations[ann.TTY] = "false"
	annotations[ann.Stdin] = ""
	annotations[ann.StdinOnce] = ""
	annotations[ann.Volumes] = ""

	return annotations
}

//GetVolumeMounts takes user provided input for bind mounts and creates Mount structs
func (c *createConfig) GetVolumeMounts() []spec.Mount {
	var m []spec.Mount
	var options []string
	for _, i := range c.volumes {
		// We need to handle SELinux options better here, specifically :Z
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		// always add rbind bc mount ignores the bind filesystem when mounting
		options = append(options, "rbind")
		m = append(m, spec.Mount{
			Destination: spliti[1],
			Type:        string(TypeBind),
			Source:      spliti[0],
			Options:     options,
		})
	}
	return m
}

//GetTmpfsMounts takes user provided input for tmpfs mounts and creates Mount structs
func (c *createConfig) GetTmpfsMounts() []spec.Mount {
	var m []spec.Mount
	for _, i := range c.tmpfs {
		// Default options if nothing passed
		options := []string{"rw", "noexec", "nosuid", "nodev", "size=65536k"}
		spliti := strings.Split(i, ":")
		destPath := spliti[0]
		if len(spliti) > 1 {
			options = strings.Split(spliti[1], ",")
		}
		m = append(m, spec.Mount{
			Destination: destPath,
			Type:        string(TypeTmpfs),
			Options:     options,
			Source:      string(TypeTmpfs),
		})
	}
	return m
}

func (c *createConfig) GetContainerCreateOptions(cli *cli.Context) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption

	// Uncomment after talking to mheon about unimplemented funcs
	// options = append(options, libpod.WithLabels(c.labels))

	if c.interactive {
		options = append(options, libpod.WithStdin())
	}
	if c.name != "" {
		logrus.Debug("appending name %s", c.name)
		options = append(options, libpod.WithName(c.name))
	}

	return options, nil
}

func getStatFromPath(path string) unix.Stat_t {
	s := unix.Stat_t{}
	_ = unix.Stat(path, &s)
	return s
}

func makeThrottleArray(throttleInput []string) ([]spec.LinuxThrottleDevice, error) {
	var ltds []spec.LinuxThrottleDevice
	for _, i := range throttleInput {
		t, err := validateBpsDevice(i)
		if err != nil {
			return []spec.LinuxThrottleDevice{}, err
		}
		ltd := spec.LinuxThrottleDevice{}
		ltd.Rate = t.rate
		ltdStat := getStatFromPath(t.path)
		ltd.Major = int64(unix.Major(ltdStat.Rdev))
		ltd.Minor = int64(unix.Major(ltdStat.Rdev))
		ltds = append(ltds, ltd)
	}
	return ltds, nil
}
