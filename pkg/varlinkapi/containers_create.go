package varlinkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/signal"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/cmd/podman/varlink"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/libpod/image"
	"github.com/projectatomic/libpod/pkg/inspect"
	cc "github.com/projectatomic/libpod/pkg/spec"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/sirupsen/logrus"
)

// CreateContainer ...
func (i *LibpodAPI) CreateContainer(call ioprojectatomicpodman.VarlinkCall, config ioprojectatomicpodman.Create) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	defer runtime.Shutdown(false)

	rtc := runtime.GetConfig()
	ctx := getContext()

	newImage, err := runtime.ImageRuntime().New(ctx, config.Image, rtc.SignaturePolicyPath, "", os.Stderr, nil, image.SigningOptions{}, false, false)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	data, err := newImage.Inspect(ctx)

	createConfig, err := varlinkCreateToCreateConfig(ctx, config, runtime, config.Image, data)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	runtimeSpec, err := cc.CreateConfigToOCISpec(createConfig)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	options, err := createConfig.GetContainerCreateOptions(runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	ctr, err := runtime.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	createConfigJSON, err := json.Marshal(createConfig)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := ctr.AddArtifact("create-config", createConfigJSON); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	logrus.Debug("new container created ", ctr.ID())

	return call.ReplyCreateContainer(ctr.ID())
}

// varlinkCreateToCreateConfig takes  the varlink input struct and maps it to a pointer
// of a CreateConfig, which eventually can be used to create the OCI spec.
func varlinkCreateToCreateConfig(ctx context.Context, create ioprojectatomicpodman.Create, runtime *libpod.Runtime, imageName string, data *inspect.ImageData) (*cc.CreateConfig, error) {
	var (
		inputCommand, command                                    []string
		memoryLimit, memoryReservation, memorySwap, memoryKernel int64
		blkioWeight                                              uint16
	)

	idmappings, err := util.ParseIDMapping(create.Uidmap, create.Gidmap, create.Subuidname, create.Subgidname)
	if err != nil {
		return nil, err
	}
	inputCommand = create.Command
	entrypoint := create.Entrypoint

	// ENTRYPOINT
	// User input entrypoint takes priority over image entrypoint
	if len(entrypoint) == 0 {
		entrypoint = data.ContainerConfig.Entrypoint
	}
	// if entrypoint=, we need to clear the entrypoint
	if len(entrypoint) == 1 && strings.Join(create.Entrypoint, "") == "" {
		entrypoint = []string{}
	}
	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
	} else if len(data.ContainerConfig.Cmd) > 0 && len(command) == 0 {
		// If not user command, add CMD
		command = append(command, data.ContainerConfig.Cmd...)
	}

	if create.Resources.Blkio_weight != 0 {
		blkioWeight = uint16(create.Resources.Blkio_weight)
	}

	stopSignal := syscall.SIGTERM
	if create.Stop_signal > 0 {
		stopSignal, err = signal.ParseSignal(fmt.Sprintf("%d", create.Stop_signal))
		if err != nil {
			return nil, err
		}
	}

	user := create.User
	if user == "" {
		user = data.ContainerConfig.User
	}

	// EXPOSED PORTS
	portBindings, err := cc.ExposedPorts(create.Exposed_ports, create.Publish, create.Publish_all, data.ContainerConfig.ExposedPorts)
	if err != nil {
		return nil, err
	}

	// NETWORK MODE
	networkMode := create.Net_mode
	if networkMode == "" {
		networkMode = "bridge"
	}

	// WORKING DIR
	workDir := create.Work_dir
	if workDir == "" {
		workDir = "/"
	}

	imageID := data.ID
	config := &cc.CreateConfig{
		Runtime:           runtime,
		BuiltinImgVolumes: data.ContainerConfig.Volumes,
		ConmonPidFile:     create.Conmon_pidfile,
		ImageVolumeType:   create.Image_volume_type,
		CapAdd:            create.Cap_add,
		CapDrop:           create.Cap_drop,
		CgroupParent:      create.Cgroup_parent,
		Command:           command,
		Detach:            create.Detach,
		Devices:           create.Devices,
		DNSOpt:            create.Dns_opt,
		DNSSearch:         create.Dns_search,
		DNSServers:        create.Dns_servers,
		Entrypoint:        create.Entrypoint,
		Env:               create.Env,
		GroupAdd:          create.Group_add,
		Hostname:          create.Hostname,
		HostAdd:           create.Host_add,
		IDMappings:        idmappings,
		Image:             imageName,
		ImageID:           imageID,
		Interactive:       create.Interactive,
		Labels:            create.Labels,
		LogDriver:         create.Log_driver,
		LogDriverOpt:      create.Log_driver_opt,
		Name:              create.Name,
		Network:           networkMode,
		IpcMode:           container.IpcMode(create.Ipc_mode),
		NetMode:           container.NetworkMode(networkMode),
		UtsMode:           container.UTSMode(create.Uts_mode),
		PidMode:           container.PidMode(create.Pid_mode),
		Pod:               create.Pod,
		Privileged:        create.Privileged,
		Publish:           create.Publish,
		PublishAll:        create.Publish_all,
		PortBindings:      portBindings,
		Quiet:             create.Quiet,
		ReadOnlyRootfs:    create.Readonly_rootfs,
		Resources: cc.CreateResourceConfig{
			BlkioWeight:       blkioWeight,
			BlkioWeightDevice: create.Resources.Blkio_weight_device,
			CPUShares:         uint64(create.Resources.Cpu_shares),
			CPUPeriod:         uint64(create.Resources.Cpu_period),
			CPUsetCPUs:        create.Resources.Cpuset_cpus,
			CPUsetMems:        create.Resources.Cpuset_mems,
			CPUQuota:          create.Resources.Cpu_quota,
			CPURtPeriod:       uint64(create.Resources.Cpu_rt_period),
			CPURtRuntime:      create.Resources.Cpu_rt_runtime,
			CPUs:              create.Resources.Cpus,
			DeviceReadBps:     create.Resources.Device_read_bps,
			DeviceReadIOps:    create.Resources.Device_write_bps,
			DeviceWriteBps:    create.Resources.Device_read_iops,
			DeviceWriteIOps:   create.Resources.Device_write_iops,
			DisableOomKiller:  create.Resources.Disable_oomkiller,
			ShmSize:           create.Resources.Shm_size,
			Memory:            memoryLimit,
			MemoryReservation: memoryReservation,
			MemorySwap:        memorySwap,
			MemorySwappiness:  int(create.Resources.Memory_swappiness),
			KernelMemory:      memoryKernel,
			OomScoreAdj:       int(create.Resources.Oom_score_adj),
			PidsLimit:         create.Resources.Pids_limit,
			Ulimit:            create.Resources.Ulimit,
		},
		Rm:          create.Rm,
		ShmDir:      create.Shm_dir,
		StopSignal:  stopSignal,
		StopTimeout: uint(create.Stop_timeout),
		Sysctl:      create.Sys_ctl,
		Tmpfs:       create.Tmpfs,
		Tty:         create.Tty,
		User:        user,
		UsernsMode:  container.UsernsMode(create.Userns_mode),
		Volumes:     create.Volumes,
		WorkDir:     workDir,
	}

	return config, nil
}
