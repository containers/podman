package common

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/spf13/pflag"
)

const sizeWithUnitFormat = "(format: `<number>[<unit>]`, where unit = b (bytes), k (kilobytes), m (megabytes), or g (gigabytes))"

var containerConfig = registry.PodmanConfig()

func GetCreateFlags(cf *ContainerCLIOpts) *pflag.FlagSet {
	createFlags := pflag.FlagSet{}
	createFlags.StringSliceVar(
		&cf.Annotation,
		"annotation", []string{},
		"Add annotations to container (key:value)",
	)
	createFlags.StringSliceVarP(
		&cf.Attach,
		"attach", "a", []string{},
		"Attach to STDIN, STDOUT or STDERR",
	)
	createFlags.StringVar(
		&cf.Authfile,
		"authfile", auth.GetDefaultAuthFile(),
		"Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override",
	)
	createFlags.StringVar(
		&cf.BlkIOWeight,
		"blkio-weight", "",
		"Block IO weight (relative weight) accepts a weight value between 10 and 1000.",
	)
	createFlags.StringSliceVar(
		&cf.BlkIOWeightDevice,
		"blkio-weight-device", []string{},
		"Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)",
	)
	createFlags.StringSliceVar(
		&cf.CapAdd,
		"cap-add", []string{},
		"Add capabilities to the container",
	)
	createFlags.StringSliceVar(
		&cf.CapDrop,
		"cap-drop", []string{},
		"Drop capabilities from the container",
	)
	createFlags.String(
		"cgroupns", "",
		"cgroup namespace to use",
	)
	createFlags.StringVar(
		&cf.CGroupsMode,
		"cgroups", containerConfig.Cgroups(),
		`control container cgroup configuration ("enabled"|"disabled"|"no-conmon"|"split")`,
	)
	createFlags.StringVar(
		&cf.CGroupParent,
		"cgroup-parent", "",
		"Optional parent cgroup for the container",
	)
	createFlags.StringVar(
		&cf.CIDFile,
		"cidfile", "",
		"Write the container ID to the file",
	)
	createFlags.StringVar(
		&cf.ConmonPIDFile,
		"conmon-pidfile", "",
		"Path to the file that will receive the PID of conmon",
	)
	createFlags.Uint64Var(
		&cf.CPUPeriod,
		"cpu-period", 0,
		"Limit the CPU CFS (Completely Fair Scheduler) period",
	)
	createFlags.Int64Var(
		&cf.CPUQuota,
		"cpu-quota", 0,
		"Limit the CPU CFS (Completely Fair Scheduler) quota",
	)
	createFlags.Uint64Var(
		&cf.CPURTPeriod,
		"cpu-rt-period", 0,
		"Limit the CPU real-time period in microseconds",
	)
	createFlags.Int64Var(
		&cf.CPURTRuntime,
		"cpu-rt-runtime", 0,
		"Limit the CPU real-time runtime in microseconds",
	)
	createFlags.Uint64Var(
		&cf.CPUShares,
		"cpu-shares", 0,
		"CPU shares (relative weight)",
	)
	createFlags.Float64Var(
		&cf.CPUS,
		"cpus", 0,
		"Number of CPUs. The default is 0.000 which means no limit",
	)
	createFlags.StringVar(
		&cf.CPUSetCPUs,
		"cpuset-cpus", "",
		"CPUs in which to allow execution (0-3, 0,1)",
	)
	createFlags.StringVar(
		&cf.CPUSetMems,
		"cpuset-mems", "",
		"Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
	)
	createFlags.StringSliceVar(
		&cf.Devices,
		"device", containerConfig.Devices(),
		fmt.Sprintf("Add a host device to the container"),
	)
	createFlags.StringSliceVar(
		&cf.DeviceCGroupRule,
		"device-cgroup-rule", []string{},
		"Add a rule to the cgroup allowed devices list",
	)
	createFlags.StringSliceVar(
		&cf.DeviceReadBPs,
		"device-read-bps", []string{},
		"Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)",
	)
	createFlags.StringSliceVar(
		&cf.DeviceReadIOPs,
		"device-read-iops", []string{},
		"Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)",
	)
	createFlags.StringSliceVar(
		&cf.DeviceWriteBPs,
		"device-write-bps", []string{},
		"Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)",
	)
	createFlags.StringSliceVar(
		&cf.DeviceWriteIOPs,
		"device-write-iops", []string{},
		"Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)",
	)
	createFlags.Bool(
		"disable-content-trust", false,
		"This is a Docker specific option and is a NOOP",
	)
	createFlags.String("entrypoint", "",
		"Overwrite the default ENTRYPOINT of the image",
	)
	createFlags.StringArrayP(
		"env", "e", containerConfig.Env(),
		"Set environment variables in container",
	)
	if !registry.IsRemote() {
		createFlags.BoolVar(
			&cf.EnvHost,
			"env-host", false, "Use all current host environment variables in container",
		)
	}
	createFlags.StringSliceVar(
		&cf.EnvFile,
		"env-file", []string{},
		"Read in a file of environment variables",
	)
	createFlags.StringSliceVar(
		&cf.Expose,
		"expose", []string{},
		"Expose a port or a range of ports",
	)
	createFlags.StringSliceVar(
		&cf.GIDMap,
		"gidmap", []string{},
		"GID map to use for the user namespace",
	)
	createFlags.StringSliceVar(
		&cf.GroupAdd,
		"group-add", []string{},
		"Add additional groups to join",
	)
	createFlags.Bool(
		"help", false, "",
	)
	createFlags.StringVar(
		&cf.HealthCmd,
		"health-cmd", "",
		"set a healthcheck command for the container ('none' disables the existing healthcheck)",
	)
	createFlags.StringVar(
		&cf.HealthInterval,
		"health-interval", DefaultHealthCheckInterval,
		"set an interval for the healthchecks (a value of disable results in no automatic timer setup)",
	)
	createFlags.UintVar(
		&cf.HealthRetries,
		"health-retries", DefaultHealthCheckRetries,
		"the number of retries allowed before a healthcheck is considered to be unhealthy",
	)
	createFlags.StringVar(
		&cf.HealthStartPeriod,
		"health-start-period", DefaultHealthCheckStartPeriod,
		"the initialization time needed for a container to bootstrap",
	)
	createFlags.StringVar(
		&cf.HealthTimeout,
		"health-timeout", DefaultHealthCheckTimeout,
		"the maximum time allowed to complete the healthcheck before an interval is considered failed",
	)
	createFlags.StringVarP(
		&cf.Hostname,
		"hostname", "h", "",
		"Set container hostname",
	)
	createFlags.BoolVar(
		&cf.HTTPProxy,
		"http-proxy", true,
		"Set proxy environment variables in the container based on the host proxy vars",
	)
	createFlags.StringVar(
		&cf.ImageVolume,
		"image-volume", DefaultImageVolume,
		`Tells podman how to handle the builtin image volumes ("bind"|"tmpfs"|"ignore")`,
	)
	createFlags.BoolVar(
		&cf.Init,
		"init", false,
		"Run an init binary inside the container that forwards signals and reaps processes",
	)
	createFlags.StringVar(
		&cf.InitPath,
		"init-path", containerConfig.InitPath(),
		// Do not use  the Value field for setting the default value to determine user input (i.e., non-empty string)
		fmt.Sprintf("Path to the container-init binary"),
	)
	createFlags.BoolVarP(
		&cf.Interactive,
		"interactive", "i", false,
		"Keep STDIN open even if not attached",
	)
	createFlags.String(
		"ipc", "",
		"IPC namespace to use",
	)
	createFlags.StringVar(
		&cf.KernelMemory,
		"kernel-memory", "",
		"Kernel memory limit "+sizeWithUnitFormat,
	)
	createFlags.StringArrayVarP(
		&cf.Label,
		"label", "l", []string{},
		"Set metadata on container",
	)
	createFlags.StringSliceVar(
		&cf.LabelFile,
		"label-file", []string{},
		"Read in a line delimited file of labels",
	)
	createFlags.StringVar(
		&cf.LogDriver,
		"log-driver", "",
		"Logging driver for the container",
	)
	createFlags.StringSliceVar(
		&cf.LogOptions,
		"log-opt", []string{},
		"Logging driver options",
	)
	createFlags.StringVarP(
		&cf.Memory,
		"memory", "m", "",
		"Memory limit "+sizeWithUnitFormat,
	)
	createFlags.StringVar(
		&cf.MemoryReservation,
		"memory-reservation", "",
		"Memory soft limit "+sizeWithUnitFormat,
	)
	createFlags.StringVar(
		&cf.MemorySwap,
		"memory-swap", "",
		"Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
	)
	createFlags.Int64Var(
		&cf.MemorySwappiness,
		"memory-swappiness", -1,
		"Tune container memory swappiness (0 to 100, or -1 for system default)",
	)
	createFlags.StringVar(
		&cf.Name,
		"name", "",
		"Assign a name to the container",
	)
	createFlags.BoolVar(
		&cf.NoHealthCheck,
		"no-healthcheck", false,
		"Disable healthchecks on container",
	)
	createFlags.BoolVar(
		&cf.OOMKillDisable,
		"oom-kill-disable", false,
		"Disable OOM Killer",
	)
	createFlags.IntVar(
		&cf.OOMScoreAdj,
		"oom-score-adj", 0,
		"Tune the host's OOM preferences (-1000 to 1000)",
	)
	createFlags.StringVar(
		&cf.OverrideArch,
		"override-arch", "",
		"use `ARCH` instead of the architecture of the machine for choosing images",
	)
	createFlags.StringVar(
		&cf.OverrideOS,
		"override-os", "",
		"use `OS` instead of the running OS for choosing images",
	)
	createFlags.StringVar(
		&cf.OverrideVariant,
		"override-variant", "",
		"Use _VARIANT_ instead of the running architecture variant for choosing images",
	)
	createFlags.String(
		"pid", "",
		"PID namespace to use",
	)
	createFlags.Int64(
		"pids-limit", containerConfig.PidsLimit(),
		"Tune container pids limit (set 0 for unlimited, -1 for server defaults)",
	)
	createFlags.StringVar(
		&cf.Pod,
		"pod", "",
		"Run container in an existing pod",
	)
	createFlags.StringVar(
		&cf.PodIDFile,
		"pod-id-file", "",
		"Read the pod ID from the file",
	)
	createFlags.BoolVar(
		&cf.Privileged,
		"privileged", false,
		"Give extended privileges to container",
	)
	createFlags.BoolVarP(
		&cf.PublishAll,
		"publish-all", "P", false,
		"Publish all exposed ports to random ports on the host interface",
	)
	createFlags.StringVar(
		&cf.Pull,
		"pull", containerConfig.Engine.PullPolicy,
		`Pull image before creating ("always"|"missing"|"never")`,
	)
	createFlags.BoolVarP(
		&cf.Quiet,
		"quiet", "q", false,
		"Suppress output information when pulling images",
	)
	createFlags.BoolVar(
		&cf.ReadOnly,
		"read-only", false,
		"Make containers root filesystem read-only",
	)
	createFlags.BoolVar(
		&cf.ReadOnlyTmpFS,
		"read-only-tmpfs", true,
		"When running containers in read-only mode mount a read-write tmpfs on /run, /tmp and /var/tmp",
	)
	createFlags.BoolVar(
		&cf.Replace,
		"replace", false,
		`If a container with the same name exists, replace it`,
	)
	createFlags.StringVar(
		&cf.Restart,
		"restart", "",
		`Restart policy to apply when a container exits ("always"|"no"|"on-failure"|"unless-stopped")`,
	)
	createFlags.BoolVar(
		&cf.Rm,
		"rm", false,
		"Remove container (and pod if created) after exit",
	)
	createFlags.BoolVar(
		&cf.RootFS,
		"rootfs", false,
		"The first argument is not an image but the rootfs to the exploded container",
	)
	createFlags.StringVar(
		&cf.SdNotifyMode,
		"sdnotify", define.SdNotifyModeContainer,
		`control sd-notify behavior ("container"|"conmon"|"ignore")`,
	)
	createFlags.StringArrayVar(
		&cf.SecurityOpt,
		"security-opt", []string{},
		"Security Options",
	)
	createFlags.String(
		"shm-size", containerConfig.ShmSize(),
		"Size of /dev/shm "+sizeWithUnitFormat,
	)
	createFlags.StringVar(
		&cf.SignaturePolicy,
		"signature-policy", "",
		"`Pathname` of signature policy file (not usually used)",
	)
	createFlags.StringVar(
		&cf.StopSignal,
		"stop-signal", "",
		"Signal to stop a container. Default is SIGTERM",
	)
	createFlags.UintVar(
		&cf.StopTimeout,
		"stop-timeout", containerConfig.Engine.StopTimeout,
		"Timeout (in seconds) to stop a container. Default is 10",
	)
	createFlags.StringSliceVar(
		&cf.StoreageOpt,
		"storage-opt", []string{},
		"Storage driver options per container",
	)
	createFlags.StringVar(
		&cf.SubUIDName,
		"subgidname", "",
		"Name of range listed in /etc/subgid for use in user namespace",
	)
	createFlags.StringVar(
		&cf.SubGIDName,
		"subuidname", "",
		"Name of range listed in /etc/subuid for use in user namespace",
	)

	createFlags.StringSliceVar(
		&cf.Sysctl,
		"sysctl", []string{},
		"Sysctl options",
	)
	createFlags.StringVar(
		&cf.Systemd,
		"systemd", "true",
		`Run container in systemd mode ("true"|"false"|"always")`,
	)
	createFlags.StringArrayVar(
		&cf.TmpFS,
		"tmpfs", []string{},
		"Mount a temporary filesystem (`tmpfs`) into a container",
	)
	createFlags.BoolVarP(
		&cf.TTY,
		"tty", "t", false,
		"Allocate a pseudo-TTY for container",
	)
	createFlags.StringVar(
		&cf.Timezone,
		"tz", containerConfig.TZ(),
		"Set timezone in container",
	)
	createFlags.StringVar(
		&cf.Umask,
		"umask", containerConfig.Umask(),
		"Set umask in container",
	)
	createFlags.StringSliceVar(
		&cf.UIDMap,
		"uidmap", []string{},
		"UID map to use for the user namespace",
	)
	createFlags.StringSliceVar(
		&cf.Ulimit,
		"ulimit", containerConfig.Ulimits(),
		"Ulimit options",
	)
	createFlags.StringVarP(
		&cf.User,
		"user", "u", "",
		"Username or UID (format: <name|uid>[:<group|gid>])",
	)
	createFlags.String(
		"userns", os.Getenv("PODMAN_USERNS"),
		"User namespace to use",
	)
	createFlags.String(
		"uts", "",
		"UTS namespace to use",
	)
	createFlags.StringArrayVar(
		&cf.Mount,
		"mount", []string{},
		"Attach a filesystem mount to the container",
	)
	createFlags.StringArrayVarP(
		&cf.Volume,
		"volume", "v", containerConfig.Volumes(),
		"Bind mount a volume into the container",
	)
	createFlags.StringArrayVar(
		&cf.VolumesFrom,
		"volumes-from", []string{},
		"Mount volumes from the specified container(s)",
	)
	createFlags.StringVarP(
		&cf.Workdir,
		"workdir", "w", "",
		"Working directory inside the container",
	)
	createFlags.StringVar(
		&cf.SeccompPolicy,
		"seccomp-policy", "default",
		"Policy for selecting a seccomp profile (experimental)",
	)
	createFlags.StringSliceVar(
		&cf.CgroupConf,
		"cgroup-conf", []string{},
		"Configure cgroup v2 (key=value)",
	)
	return &createFlags
}
