package common

import (
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

const sizeWithUnitFormat = "(format: `<number>[<unit>]`, where unit = b (bytes), k (kilobytes), m (megabytes), or g (gigabytes))"

var containerConfig = registry.PodmanConfig()

func DefineCreateFlags(cmd *cobra.Command, cf *entities.ContainerCreateOptions, isInfra bool) {
	createFlags := cmd.Flags()

	if !isInfra {
		annotationFlagName := "annotation"
		createFlags.StringSliceVar(
			&cf.Annotation,
			annotationFlagName, []string{},
			"Add annotations to container (key:value)",
		)
		_ = cmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

		attachFlagName := "attach"
		createFlags.StringSliceVarP(
			&cf.Attach,
			attachFlagName, "a", []string{},
			"Attach to STDIN, STDOUT or STDERR",
		)
		_ = cmd.RegisterFlagCompletionFunc(attachFlagName, AutocompleteCreateAttach)

		authfileFlagName := "authfile"
		createFlags.StringVar(
			&cf.Authfile,
			authfileFlagName, auth.GetDefaultAuthFile(),
			"Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override",
		)
		_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

		blkioWeightFlagName := "blkio-weight"
		createFlags.StringVar(
			&cf.BlkIOWeight,
			blkioWeightFlagName, "",
			"Block IO weight (relative weight) accepts a weight value between 10 and 1000.",
		)
		_ = cmd.RegisterFlagCompletionFunc(blkioWeightFlagName, completion.AutocompleteNone)

		blkioWeightDeviceFlagName := "blkio-weight-device"
		createFlags.StringSliceVar(
			&cf.BlkIOWeightDevice,
			blkioWeightDeviceFlagName, []string{},
			"Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)",
		)
		_ = cmd.RegisterFlagCompletionFunc(blkioWeightDeviceFlagName, completion.AutocompleteDefault)

		capAddFlagName := "cap-add"
		createFlags.StringSliceVar(
			&cf.CapAdd,
			capAddFlagName, []string{},
			"Add capabilities to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(capAddFlagName, completion.AutocompleteCapabilities)

		capDropFlagName := "cap-drop"
		createFlags.StringSliceVar(
			&cf.CapDrop,
			capDropFlagName, []string{},
			"Drop capabilities from the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(capDropFlagName, completion.AutocompleteCapabilities)

		cgroupnsFlagName := "cgroupns"
		createFlags.String(
			cgroupnsFlagName, "",
			"cgroup namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupnsFlagName, AutocompleteNamespace)

		cgroupsFlagName := "cgroups"
		createFlags.StringVar(
			&cf.CGroupsMode,
			cgroupsFlagName, cgroupConfig(),
			`control container cgroup configuration ("enabled"|"disabled"|"no-conmon"|"split")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupsFlagName, AutocompleteCgroupMode)

		cpusFlagName := "cpus"
		createFlags.Float64Var(
			&cf.CPUS,
			cpusFlagName, 0,
			"Number of CPUs. The default is 0.000 which means no limit",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

		cpusetCpusFlagName := "cpuset-cpus"
		createFlags.StringVar(
			&cf.CPUSetCPUs,
			cpusetCpusFlagName, "",
			"CPUs in which to allow execution (0-3, 0,1)",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpusetCpusFlagName, completion.AutocompleteNone)

		cpuPeriodFlagName := "cpu-period"
		createFlags.Uint64Var(
			&cf.CPUPeriod,
			cpuPeriodFlagName, 0,
			"Limit the CPU CFS (Completely Fair Scheduler) period",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuPeriodFlagName, completion.AutocompleteNone)

		cpuQuotaFlagName := "cpu-quota"
		createFlags.Int64Var(
			&cf.CPUQuota,
			cpuQuotaFlagName, 0,
			"Limit the CPU CFS (Completely Fair Scheduler) quota",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuQuotaFlagName, completion.AutocompleteNone)

		cpuRtPeriodFlagName := "cpu-rt-period"
		createFlags.Uint64Var(
			&cf.CPURTPeriod,
			cpuRtPeriodFlagName, 0,
			"Limit the CPU real-time period in microseconds",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuRtPeriodFlagName, completion.AutocompleteNone)

		cpuRtRuntimeFlagName := "cpu-rt-runtime"
		createFlags.Int64Var(
			&cf.CPURTRuntime,
			cpuRtRuntimeFlagName, 0,
			"Limit the CPU real-time runtime in microseconds",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuRtRuntimeFlagName, completion.AutocompleteNone)

		cpuSharesFlagName := "cpu-shares"
		createFlags.Uint64Var(
			&cf.CPUShares,
			cpuSharesFlagName, 0,
			"CPU shares (relative weight)",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuSharesFlagName, completion.AutocompleteNone)
		cidfileFlagName := "cidfile"
		createFlags.StringVar(
			&cf.CIDFile,
			cidfileFlagName, "",
			"Write the container ID to the file",
		)
		_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)
		cpusetMemsFlagName := "cpuset-mems"
		createFlags.StringVar(
			&cf.CPUSetMems,
			cpusetMemsFlagName, "",
			"Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpusetMemsFlagName, completion.AutocompleteNone)

		deviceFlagName := "device"
		createFlags.StringSliceVar(
			&cf.Devices,
			deviceFlagName, devices(),
			"Add a host device to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceFlagName, completion.AutocompleteDefault)

		deviceCgroupRuleFlagName := "device-cgroup-rule"
		createFlags.StringSliceVar(
			&cf.DeviceCGroupRule,
			deviceCgroupRuleFlagName, []string{},
			"Add a rule to the cgroup allowed devices list",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceCgroupRuleFlagName, completion.AutocompleteNone)

		deviceReadBpsFlagName := "device-read-bps"
		createFlags.StringSliceVar(
			&cf.DeviceReadBPs,
			deviceReadBpsFlagName, []string{},
			"Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceReadBpsFlagName, completion.AutocompleteDefault)

		deviceReadIopsFlagName := "device-read-iops"
		createFlags.StringSliceVar(
			&cf.DeviceReadIOPs,
			deviceReadIopsFlagName, []string{},
			"Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceReadIopsFlagName, completion.AutocompleteDefault)

		deviceWriteBpsFlagName := "device-write-bps"
		createFlags.StringSliceVar(
			&cf.DeviceWriteBPs,
			deviceWriteBpsFlagName, []string{},
			"Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceWriteBpsFlagName, completion.AutocompleteDefault)

		deviceWriteIopsFlagName := "device-write-iops"
		createFlags.StringSliceVar(
			&cf.DeviceWriteIOPs,
			deviceWriteIopsFlagName, []string{},
			"Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceWriteIopsFlagName, completion.AutocompleteDefault)

		createFlags.Bool(
			"disable-content-trust", false,
			"This is a Docker specific option and is a NOOP",
		)

		envFlagName := "env"
		createFlags.StringArrayP(
			envFlagName, "e", env(),
			"Set environment variables in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(envFlagName, completion.AutocompleteNone)

		if !registry.IsRemote() {
			createFlags.BoolVar(
				&cf.EnvHost,
				"env-host", false, "Use all current host environment variables in container",
			)
		}

		envFileFlagName := "env-file"
		createFlags.StringSliceVar(
			&cf.EnvFile,
			envFileFlagName, []string{},
			"Read in a file of environment variables",
		)
		_ = cmd.RegisterFlagCompletionFunc(envFileFlagName, completion.AutocompleteDefault)

		exposeFlagName := "expose"
		createFlags.StringSliceVar(
			&cf.Expose,
			exposeFlagName, []string{},
			"Expose a port or a range of ports",
		)
		_ = cmd.RegisterFlagCompletionFunc(exposeFlagName, completion.AutocompleteNone)

		groupAddFlagName := "group-add"
		createFlags.StringSliceVar(
			&cf.GroupAdd,
			groupAddFlagName, []string{},
			"Add additional groups to the primary container process. 'keep-groups' allows container processes to use supplementary groups.",
		)
		_ = cmd.RegisterFlagCompletionFunc(groupAddFlagName, completion.AutocompleteNone)

		healthCmdFlagName := "health-cmd"
		createFlags.StringVar(
			&cf.HealthCmd,
			healthCmdFlagName, "",
			"set a healthcheck command for the container ('none' disables the existing healthcheck)",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthCmdFlagName, completion.AutocompleteNone)

		healthIntervalFlagName := "health-interval"
		createFlags.StringVar(
			&cf.HealthInterval,
			healthIntervalFlagName, DefaultHealthCheckInterval,
			"set an interval for the healthchecks (a value of disable results in no automatic timer setup)",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthIntervalFlagName, completion.AutocompleteNone)

		healthRetriesFlagName := "health-retries"
		createFlags.UintVar(
			&cf.HealthRetries,
			healthRetriesFlagName, DefaultHealthCheckRetries,
			"the number of retries allowed before a healthcheck is considered to be unhealthy",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthRetriesFlagName, completion.AutocompleteNone)

		healthStartPeriodFlagName := "health-start-period"
		createFlags.StringVar(
			&cf.HealthStartPeriod,
			healthStartPeriodFlagName, DefaultHealthCheckStartPeriod,
			"the initialization time needed for a container to bootstrap",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthStartPeriodFlagName, completion.AutocompleteNone)

		healthTimeoutFlagName := "health-timeout"
		createFlags.StringVar(
			&cf.HealthTimeout,
			healthTimeoutFlagName, DefaultHealthCheckTimeout,
			"the maximum time allowed to complete the healthcheck before an interval is considered failed",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthTimeoutFlagName, completion.AutocompleteNone)

		createFlags.BoolVar(
			&cf.HTTPProxy,
			"http-proxy", containerConfig.Containers.HTTPProxy,
			"Set proxy environment variables in the container based on the host proxy vars",
		)

		imageVolumeFlagName := "image-volume"
		createFlags.StringVar(
			&cf.ImageVolume,
			imageVolumeFlagName, DefaultImageVolume,
			`Tells podman how to handle the builtin image volumes ("bind"|"tmpfs"|"ignore")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(imageVolumeFlagName, AutocompleteImageVolume)

		createFlags.BoolVar(
			&cf.Init,
			"init", false,
			"Run an init binary inside the container that forwards signals and reaps processes",
		)

		initPathFlagName := "init-path"
		createFlags.StringVar(
			&cf.InitPath,
			initPathFlagName, initPath(),
			// Do not use  the Value field for setting the default value to determine user input (i.e., non-empty string)
			"Path to the container-init binary",
		)
		_ = cmd.RegisterFlagCompletionFunc(initPathFlagName, completion.AutocompleteDefault)

		createFlags.BoolVarP(
			&cf.Interactive,
			"interactive", "i", false,
			"Keep STDIN open even if not attached",
		)
		ipcFlagName := "ipc"
		createFlags.String(
			ipcFlagName, "",
			"IPC namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(ipcFlagName, AutocompleteNamespace)

		kernelMemoryFlagName := "kernel-memory"
		createFlags.StringVar(
			&cf.KernelMemory,
			kernelMemoryFlagName, "",
			"Kernel memory limit "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(kernelMemoryFlagName, completion.AutocompleteNone)
		logDriverFlagName := "log-driver"
		createFlags.StringVar(
			&cf.LogDriver,
			logDriverFlagName, logDriver(),
			"Logging driver for the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(logDriverFlagName, AutocompleteLogDriver)

		logOptFlagName := "log-opt"
		createFlags.StringSliceVar(
			&cf.LogOptions,
			logOptFlagName, []string{},
			"Logging driver options",
		)
		_ = cmd.RegisterFlagCompletionFunc(logOptFlagName, AutocompleteLogOpt)

		memoryFlagName := "memory"
		createFlags.StringVarP(
			&cf.Memory,
			memoryFlagName, "m", "",
			"Memory limit "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

		memoryReservationFlagName := "memory-reservation"
		createFlags.StringVar(
			&cf.MemoryReservation,
			memoryReservationFlagName, "",
			"Memory soft limit "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(memoryReservationFlagName, completion.AutocompleteNone)

		memorySwapFlagName := "memory-swap"
		createFlags.StringVar(
			&cf.MemorySwap,
			memorySwapFlagName, "",
			"Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
		)
		_ = cmd.RegisterFlagCompletionFunc(memorySwapFlagName, completion.AutocompleteNone)

		memorySwappinessFlagName := "memory-swappiness"
		createFlags.Int64Var(
			&cf.MemorySwappiness,
			memorySwappinessFlagName, -1,
			"Tune container memory swappiness (0 to 100, or -1 for system default)",
		)
		_ = cmd.RegisterFlagCompletionFunc(memorySwappinessFlagName, completion.AutocompleteNone)

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

		oomScoreAdjFlagName := "oom-score-adj"
		createFlags.IntVar(
			&cf.OOMScoreAdj,
			oomScoreAdjFlagName, 0,
			"Tune the host's OOM preferences (-1000 to 1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(oomScoreAdjFlagName, completion.AutocompleteNone)

		archFlagName := "arch"
		createFlags.StringVar(
			&cf.Arch,
			archFlagName, "",
			"use `ARCH` instead of the architecture of the machine for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

		osFlagName := "os"
		createFlags.StringVar(
			&cf.OS,
			osFlagName, "",
			"use `OS` instead of the running OS for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

		variantFlagName := "variant"
		createFlags.StringVar(
			&cf.Variant,
			variantFlagName, "",
			"Use `VARIANT` instead of the running architecture variant for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

		pidsLimitFlagName := "pids-limit"
		createFlags.Int64(
			pidsLimitFlagName, pidsLimit(),
			"Tune container pids limit (set -1 for unlimited)",
		)
		_ = cmd.RegisterFlagCompletionFunc(pidsLimitFlagName, completion.AutocompleteNone)

		platformFlagName := "platform"
		createFlags.StringVar(
			&cf.Platform,
			platformFlagName, "",
			"Specify the platform for selecting the image.  (Conflicts with --arch and --os)",
		)
		_ = cmd.RegisterFlagCompletionFunc(platformFlagName, completion.AutocompleteNone)

		podFlagName := "pod"
		createFlags.StringVar(
			&cf.Pod,
			podFlagName, "",
			"Run container in an existing pod",
		)
		_ = cmd.RegisterFlagCompletionFunc(podFlagName, AutocompletePods)

		podIDFileFlagName := "pod-id-file"
		createFlags.StringVar(
			&cf.PodIDFile,
			podIDFileFlagName, "",
			"Read the pod ID from the file",
		)
		_ = cmd.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)
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

		pullFlagName := "pull"
		createFlags.StringVar(
			&cf.Pull,
			pullFlagName, policy(),
			`Pull image before creating ("always"|"missing"|"never")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(pullFlagName, AutocompletePullOption)

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
		requiresFlagName := "requires"
		createFlags.StringSliceVar(
			&cf.Requires,
			requiresFlagName, []string{},
			"Add one or more requirement containers that must be started before this container will start",
		)
		_ = cmd.RegisterFlagCompletionFunc(requiresFlagName, AutocompleteContainers)

		restartFlagName := "restart"
		createFlags.StringVar(
			&cf.Restart,
			restartFlagName, "",
			`Restart policy to apply when a container exits ("always"|"no"|"on-failure"|"unless-stopped")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(restartFlagName, AutocompleteRestartOption)

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

		sdnotifyFlagName := "sdnotify"
		createFlags.StringVar(
			&cf.SdNotifyMode,
			sdnotifyFlagName, define.SdNotifyModeContainer,
			`control sd-notify behavior ("container"|"conmon"|"ignore")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(sdnotifyFlagName, AutocompleteSDNotify)

		secretFlagName := "secret"
		createFlags.StringArrayVar(
			&cf.Secrets,
			secretFlagName, []string{},
			"Add secret to container",
		)
		_ = cmd.RegisterFlagCompletionFunc(secretFlagName, AutocompleteSecrets)

		securityOptFlagName := "security-opt"
		createFlags.StringArrayVar(
			&cf.SecurityOpt,
			securityOptFlagName, []string{},
			"Security Options",
		)
		_ = cmd.RegisterFlagCompletionFunc(securityOptFlagName, AutocompleteSecurityOption)

		shmSizeFlagName := "shm-size"
		createFlags.String(
			shmSizeFlagName, shmSize(),
			"Size of /dev/shm "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(shmSizeFlagName, completion.AutocompleteNone)

		stopSignalFlagName := "stop-signal"
		createFlags.StringVar(
			&cf.SignaturePolicy,
			"signature-policy", "",
			"`Pathname` of signature policy file (not usually used)",
		)
		createFlags.StringVar(
			&cf.StopSignal,
			stopSignalFlagName, "",
			"Signal to stop a container. Default is SIGTERM",
		)
		_ = cmd.RegisterFlagCompletionFunc(stopSignalFlagName, AutocompleteStopSignal)

		stopTimeoutFlagName := "stop-timeout"
		createFlags.UintVar(
			&cf.StopTimeout,
			stopTimeoutFlagName, containerConfig.Engine.StopTimeout,
			"Timeout (in seconds) that containers stopped by user command have to exit. If exceeded, the container will be forcibly stopped via SIGKILL.",
		)
		_ = cmd.RegisterFlagCompletionFunc(stopTimeoutFlagName, completion.AutocompleteNone)

		sysctlFlagName := "sysctl"
		createFlags.StringSliceVar(
			&cf.Sysctl,
			sysctlFlagName, []string{},
			"Sysctl options",
		)
		//TODO: Add function for sysctl completion.
		_ = cmd.RegisterFlagCompletionFunc(sysctlFlagName, completion.AutocompleteNone)

		systemdFlagName := "systemd"
		createFlags.StringVar(
			&cf.Systemd,
			systemdFlagName, "true",
			`Run container in systemd mode ("true"|"false"|"always")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(systemdFlagName, AutocompleteSystemdFlag)

		personalityFlagName := "personality"
		createFlags.StringVar(
			&cf.Personality,
			personalityFlagName, "",
			"Configure execution domain using personality (e.g., LINUX/LINUX32)",
		)
		_ = cmd.RegisterFlagCompletionFunc(personalityFlagName, AutocompleteNamespace)

		timeoutFlagName := "timeout"
		createFlags.UintVar(
			&cf.Timeout,
			timeoutFlagName, 0,
			"Maximum length of time a container is allowed to run. The container will be killed automatically after the time expires.",
		)
		_ = cmd.RegisterFlagCompletionFunc(timeoutFlagName, completion.AutocompleteNone)

		commonFlag.OptionalBoolFlag(createFlags,
			&cf.TLSVerify,
			"tls-verify",
			"Require HTTPS and verify certificates when contacting registries for pulling images",
		)

		tmpfsFlagName := "tmpfs"
		createFlags.StringArrayVar(
			&cf.TmpFS,
			tmpfsFlagName, []string{},
			"Mount a temporary filesystem (`tmpfs`) into a container",
		)
		_ = cmd.RegisterFlagCompletionFunc(tmpfsFlagName, completion.AutocompleteDefault)

		createFlags.BoolVarP(
			&cf.TTY,
			"tty", "t", false,
			"Allocate a pseudo-TTY for container",
		)

		timezoneFlagName := "tz"
		createFlags.StringVar(
			&cf.Timezone,
			timezoneFlagName, containerConfig.TZ(),
			"Set timezone in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(timezoneFlagName, completion.AutocompleteNone) //TODO: add timezone completion

		umaskFlagName := "umask"
		createFlags.StringVar(
			&cf.Umask,
			umaskFlagName, containerConfig.Umask(),
			"Set umask in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(umaskFlagName, completion.AutocompleteNone)

		ulimitFlagName := "ulimit"
		createFlags.StringSliceVar(
			&cf.Ulimit,
			ulimitFlagName, ulimits(),
			"Ulimit options",
		)
		_ = cmd.RegisterFlagCompletionFunc(ulimitFlagName, completion.AutocompleteNone)

		userFlagName := "user"
		createFlags.StringVarP(
			&cf.User,
			userFlagName, "u", "",
			"Username or UID (format: <name|uid>[:<group|gid>])",
		)
		_ = cmd.RegisterFlagCompletionFunc(userFlagName, AutocompleteUserFlag)

		utsFlagName := "uts"
		createFlags.String(
			utsFlagName, "",
			"UTS namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(utsFlagName, AutocompleteNamespace)

		mountFlagName := "mount"
		createFlags.StringArrayVar(
			&cf.Mount,
			mountFlagName, []string{},
			"Attach a filesystem mount to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(mountFlagName, AutocompleteMountFlag)

		volumeDesciption := "Bind mount a volume into the container"
		if registry.IsRemote() {
			volumeDesciption = "Bind mount a volume into the container. Volume src will be on the server machine, not the client"
		}
		volumeFlagName := "volume"
		createFlags.StringArrayVarP(
			&cf.Volume,
			volumeFlagName, "v", volumes(),
			volumeDesciption,
		)
		_ = cmd.RegisterFlagCompletionFunc(volumeFlagName, AutocompleteVolumeFlag)

		volumesFromFlagName := "volumes-from"
		createFlags.StringArrayVar(
			&cf.VolumesFrom,
			volumesFromFlagName, []string{},
			"Mount volumes from the specified container(s)",
		)
		_ = cmd.RegisterFlagCompletionFunc(volumesFromFlagName, AutocompleteContainers)

		workdirFlagName := "workdir"
		createFlags.StringVarP(
			&cf.Workdir,
			workdirFlagName, "w", "",
			"Working directory inside the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(workdirFlagName, completion.AutocompleteDefault)

		seccompPolicyFlagName := "seccomp-policy"
		createFlags.StringVar(
			&cf.SeccompPolicy,
			seccompPolicyFlagName, "default",
			"Policy for selecting a seccomp profile (experimental)",
		)
		_ = cmd.RegisterFlagCompletionFunc(seccompPolicyFlagName, completion.AutocompleteDefault)

		cgroupConfFlagName := "cgroup-conf"
		createFlags.StringSliceVar(
			&cf.CgroupConf,
			cgroupConfFlagName, []string{},
			"Configure cgroup v2 (key=value)",
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupConfFlagName, completion.AutocompleteNone)

		pidFileFlagName := "pidfile"
		createFlags.StringVar(
			&cf.PidFile,
			pidFileFlagName, "",
			"Write the container process ID to the file")
		_ = cmd.RegisterFlagCompletionFunc(pidFileFlagName, completion.AutocompleteDefault)

		_ = createFlags.MarkHidden("signature-policy")
		if registry.IsRemote() {
			_ = createFlags.MarkHidden("env-host")
			_ = createFlags.MarkHidden("http-proxy")
		}

		createFlags.BoolVar(
			&cf.Replace,
			"replace", false,
			`If a container with the same name exists, replace it`,
		)
	}

	subgidnameFlagName := "subgidname"
	createFlags.StringVar(
		&cf.SubUIDName,
		subgidnameFlagName, "",
		"Name of range listed in /etc/subgid for use in user namespace",
	)
	_ = cmd.RegisterFlagCompletionFunc(subgidnameFlagName, completion.AutocompleteSubgidName)

	subuidnameFlagName := "subuidname"
	createFlags.StringVar(
		&cf.SubGIDName,
		subuidnameFlagName, "",
		"Name of range listed in /etc/subuid for use in user namespace",
	)
	_ = cmd.RegisterFlagCompletionFunc(subuidnameFlagName, completion.AutocompleteSubuidName)

	gidmapFlagName := "gidmap"
	createFlags.StringSliceVar(
		&cf.GIDMap,
		gidmapFlagName, []string{},
		"GID map to use for the user namespace",
	)
	_ = cmd.RegisterFlagCompletionFunc(gidmapFlagName, completion.AutocompleteNone)

	uidmapFlagName := "uidmap"
	createFlags.StringSliceVar(
		&cf.UIDMap,
		uidmapFlagName, []string{},
		"UID map to use for the user namespace",
	)
	_ = cmd.RegisterFlagCompletionFunc(uidmapFlagName, completion.AutocompleteNone)

	usernsFlagName := "userns"
	createFlags.String(
		usernsFlagName, os.Getenv("PODMAN_USERNS"),
		"User namespace to use",
	)
	_ = cmd.RegisterFlagCompletionFunc(usernsFlagName, AutocompleteUserNamespace)

	cgroupParentFlagName := "cgroup-parent"
	createFlags.StringVar(
		&cf.CGroupParent,
		cgroupParentFlagName, "",
		"Optional parent cgroup for the container",
	)
	_ = cmd.RegisterFlagCompletionFunc(cgroupParentFlagName, completion.AutocompleteDefault)

	conmonPidfileFlagName := ""
	if !isInfra {
		conmonPidfileFlagName = "conmon-pidfile"
	} else {
		conmonPidfileFlagName = "infra-conmon-pidfile"
	}
	createFlags.StringVar(
		&cf.ConmonPIDFile,
		conmonPidfileFlagName, "",
		"Path to the file that will receive the PID of conmon",
	)
	_ = cmd.RegisterFlagCompletionFunc(conmonPidfileFlagName, completion.AutocompleteDefault)

	entrypointFlagName := ""
	if !isInfra {
		entrypointFlagName = "entrypoint"
	} else {
		entrypointFlagName = "infra-command"
	}

	createFlags.String(entrypointFlagName, "",
		"Overwrite the default ENTRYPOINT of the image",
	)
	_ = cmd.RegisterFlagCompletionFunc(entrypointFlagName, completion.AutocompleteNone)

	hostnameFlagName := "hostname"
	createFlags.StringVarP(
		&cf.Hostname,
		hostnameFlagName, "h", "",
		"Set container hostname",
	)
	_ = cmd.RegisterFlagCompletionFunc(hostnameFlagName, completion.AutocompleteNone)

	labelFlagName := "label"
	createFlags.StringArrayVarP(
		&cf.Label,
		labelFlagName, "l", []string{},
		"Set metadata on container",
	)
	_ = cmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)

	labelFileFlagName := "label-file"
	createFlags.StringSliceVar(
		&cf.LabelFile,
		labelFileFlagName, []string{},
		"Read in a line delimited file of labels",
	)
	_ = cmd.RegisterFlagCompletionFunc(labelFileFlagName, completion.AutocompleteDefault)

	nameFlagName := ""
	if !isInfra {
		nameFlagName = "name"
		createFlags.StringVar(
			&cf.Name,
			nameFlagName, "",
			"Assign a name to the container",
		)
	} else {
		nameFlagName = "infra-name"
		createFlags.StringVar(
			&cf.Name,
			nameFlagName, "",
			"Assign a name to the container",
		)
	}
	_ = cmd.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

	createFlags.Bool(
		"help", false, "",
	)

	pidFlagName := "pid"
	createFlags.StringVar(
		&cf.PID,
		pidFlagName, "",
		"PID namespace to use",
	)
	_ = cmd.RegisterFlagCompletionFunc(pidFlagName, AutocompleteNamespace)
}
