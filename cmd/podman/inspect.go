package main

import (
	"encoding/json"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/formats"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	inspectTypeContainer = "container"
	inspectTypeImage     = "image"
	inspectAll           = "all"
)

var (
	inspectFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: inspectAll,
			Usage: "Return JSON for specified type, (e.g image, container or task)",
		},
		cli.StringFlag{
			Name:  "format, f",
			Usage: "Change the output format to a Go template",
		},
		cli.BoolFlag{
			Name:  "size",
			Usage: "Display total file size if the type is container",
		},
		LatestFlag,
	}
	inspectDescription = "This displays the low-level information on containers and images identified by name or ID. By default, this will render all results in a JSON array. If the container and image have the same name, this will return container JSON for unspecified type."
	inspectCommand     = cli.Command{
		Name:        "inspect",
		Usage:       "Displays the configuration of a container or image",
		Description: inspectDescription,
		Flags:       inspectFlags,
		Action:      inspectCmd,
		ArgsUsage:   "CONTAINER-OR-IMAGE [CONTAINER-OR-IMAGE]...",
	}
)

func inspectCmd(c *cli.Context) error {
	args := c.Args()
	inspectType := c.String("type")
	latestContainer := c.Bool("latest")
	if len(args) == 0 && !latestContainer {
		return errors.Errorf("container or image name must be specified: podman inspect [options [...]] name")
	}

	if len(args) > 0 && latestContainer {
		return errors.Errorf("you cannot provide additional arguements with --latest")
	}
	if err := validateFlags(c, inspectFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if !libpod.StringInSlice(inspectType, []string{inspectTypeContainer, inspectTypeImage, inspectAll}) {
		return errors.Errorf("the only recognized types are %q, %q, and %q", inspectTypeContainer, inspectTypeImage, inspectAll)
	}

	outputFormat := c.String("format")
	if strings.Contains(outputFormat, "{{.Id}}") {
		outputFormat = strings.Replace(outputFormat, "{{.Id}}", formats.IDString, -1)
	}
	if latestContainer {
		lc, err := runtime.GetLatestContainer()
		if err != nil {
			return err
		}
		args = append(args, lc.ID())
		inspectType = inspectTypeContainer
	}

	inspectedObjects, iterateErr := iterateInput(c, args, runtime, inspectType)

	var out formats.Writer
	if outputFormat != "" && outputFormat != formats.JSONString {
		//template
		out = formats.StdoutTemplateArray{Output: inspectedObjects, Template: outputFormat}
	} else {
		// default is json output
		out = formats.JSONStructArray{Output: inspectedObjects}
	}

	formats.Writer(out).Out()
	return iterateErr
}

// func iterateInput iterates the images|containers the user has requested and returns the inspect data and error
func iterateInput(c *cli.Context, args []string, runtime *libpod.Runtime, inspectType string) ([]interface{}, error) {
	var (
		data           interface{}
		inspectedItems []interface{}
		inspectError   error
	)

	for _, input := range args {
		switch inspectType {
		case inspectTypeContainer:
			ctr, err := runtime.LookupContainer(input)
			if err != nil {
				inspectError = errors.Wrapf(err, "error looking up container %q", input)
				break
			}
			libpodInspectData, err := ctr.Inspect(c.Bool("size"))
			if err != nil {
				inspectError = errors.Wrapf(err, "error getting libpod container inspect data %q", ctr.ID)
				break
			}
			data, err = getCtrInspectInfo(ctr, libpodInspectData)
			if err != nil {
				inspectError = errors.Wrapf(err, "error parsing container data %q", ctr.ID())
				break
			}
		case inspectTypeImage:
			newImage := runtime.NewImage(input)
			newImage.GetLocalImageName()
			image, err := runtime.GetImage(newImage.LocalName)
			if err != nil {
				inspectError = errors.Wrapf(err, "error getting image %q", input)
				break
			}
			data, err = runtime.GetImageInspectInfo(*image)
			if err != nil {
				inspectError = errors.Wrapf(err, "error parsing image data %q", image.ID)
				break
			}
		case inspectAll:
			ctr, err := runtime.LookupContainer(input)
			if err != nil {
				newImage := runtime.NewImage(input)
				newImage.GetLocalImageName()
				image, err := runtime.GetImage(newImage.LocalName)
				if err != nil {
					inspectError = errors.Wrapf(err, "error getting image %q", input)
					break
				}
				data, err = runtime.GetImageInspectInfo(*image)
				if err != nil {
					inspectError = errors.Wrapf(err, "error parsing image data %q", image.ID)
					break
				}
			} else {
				libpodInspectData, err := ctr.Inspect(c.Bool("size"))
				if err != nil {
					inspectError = errors.Wrapf(err, "error getting libpod container inspect data %q", ctr.ID)
					break
				}
				data, err = getCtrInspectInfo(ctr, libpodInspectData)
				if err != nil {
					inspectError = errors.Wrapf(err, "error parsing container data %q", ctr.ID)
					break
				}
			}
		}
		if inspectError == nil {
			inspectedItems = append(inspectedItems, data)
		}
	}
	return inspectedItems, inspectError
}

func getCtrInspectInfo(ctr *libpod.Container, ctrInspectData *inspect.ContainerInspectData) (*inspect.ContainerData, error) {
	config := ctr.Config()
	spec := config.Spec

	cpus, mems, period, quota, realtimePeriod, realtimeRuntime, shares := getCPUInfo(spec)
	blkioWeight, blkioWeightDevice, blkioReadBps, blkioWriteBps, blkioReadIOPS, blkioeWriteIOPS := getBLKIOInfo(spec)
	memKernel, memReservation, memSwap, memSwappiness, memDisableOOMKiller := getMemoryInfo(spec)
	pidsLimit := getPidsInfo(spec)
	cgroup := getCgroup(spec)

	var createArtifact createConfig
	artifact, err := ctr.GetArtifact("create-config")
	if err == nil {
		if err := json.Unmarshal(artifact, &createArtifact); err != nil {
			return nil, err
		}
	} else {
		logrus.Errorf("couldn't get some inspect information, error getting artifact %q: %v", ctr.ID(), err)
	}

	data := &inspect.ContainerData{
		ctrInspectData,
		&inspect.HostConfig{
			ConsoleSize:          spec.Process.ConsoleSize,
			OomScoreAdj:          spec.Process.OOMScoreAdj,
			CPUShares:            shares,
			BlkioWeight:          blkioWeight,
			BlkioWeightDevice:    blkioWeightDevice,
			BlkioDeviceReadBps:   blkioReadBps,
			BlkioDeviceWriteBps:  blkioWriteBps,
			BlkioDeviceReadIOps:  blkioReadIOPS,
			BlkioDeviceWriteIOps: blkioeWriteIOPS,
			CPUPeriod:            period,
			CPUQuota:             quota,
			CPURealtimePeriod:    realtimePeriod,
			CPURealtimeRuntime:   realtimeRuntime,
			CPUSetCPUs:           cpus,
			CPUSetMems:           mems,
			Devices:              spec.Linux.Devices,
			KernelMemory:         memKernel,
			MemoryReservation:    memReservation,
			MemorySwap:           memSwap,
			MemorySwappiness:     memSwappiness,
			OomKillDisable:       memDisableOOMKiller,
			PidsLimit:            pidsLimit,
			Privileged:           config.Privileged,
			ReadonlyRootfs:       spec.Root.Readonly,
			Runtime:              ctr.RuntimeName(),
			NetworkMode:          string(createArtifact.NetMode),
			IpcMode:              string(createArtifact.IpcMode),
			Cgroup:               cgroup,
			UTSMode:              string(createArtifact.UtsMode),
			UsernsMode:           createArtifact.NsUser,
			GroupAdd:             spec.Process.User.AdditionalGids,
			ContainerIDFile:      createArtifact.CidFile,
			AutoRemove:           createArtifact.Rm,
			CapAdd:               createArtifact.CapAdd,
			CapDrop:              createArtifact.CapDrop,
			DNS:                  createArtifact.DNSServers,
			DNSOptions:           createArtifact.DNSOpt,
			DNSSearch:            createArtifact.DNSSearch,
			PidMode:              string(createArtifact.PidMode),
			CgroupParent:         createArtifact.CgroupParent,
			ShmSize:              createArtifact.Resources.ShmSize,
			Memory:               createArtifact.Resources.Memory,
			Ulimits:              createArtifact.Resources.Ulimit,
			SecurityOpt:          createArtifact.SecurityOpts,
		},
		&inspect.CtrConfig{
			Hostname:    spec.Hostname,
			User:        spec.Process.User,
			Env:         spec.Process.Env,
			Image:       config.RootfsImageName,
			WorkingDir:  spec.Process.Cwd,
			Labels:      config.Labels,
			Annotations: spec.Annotations,
			Tty:         spec.Process.Terminal,
			OpenStdin:   config.Stdin,
			StopSignal:  config.StopSignal,
			Cmd:         config.Spec.Process.Args,
			Entrypoint:  strings.Join(createArtifact.Entrypoint, " "),
		},
	}
	return data, nil
}

func getCPUInfo(spec *specs.Spec) (string, string, *uint64, *int64, *uint64, *int64, *uint64) {
	if spec.Linux.Resources == nil {
		return "", "", nil, nil, nil, nil, nil
	}
	cpu := spec.Linux.Resources.CPU
	if cpu == nil {
		return "", "", nil, nil, nil, nil, nil
	}
	return cpu.Cpus, cpu.Mems, cpu.Period, cpu.Quota, cpu.RealtimePeriod, cpu.RealtimeRuntime, cpu.Shares
}

func getBLKIOInfo(spec *specs.Spec) (*uint16, []specs.LinuxWeightDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice) {
	if spec.Linux.Resources == nil {
		return nil, nil, nil, nil, nil, nil
	}
	blkio := spec.Linux.Resources.BlockIO
	if blkio == nil {
		return nil, nil, nil, nil, nil, nil
	}
	return blkio.Weight, blkio.WeightDevice, blkio.ThrottleReadBpsDevice, blkio.ThrottleWriteBpsDevice, blkio.ThrottleReadIOPSDevice, blkio.ThrottleWriteIOPSDevice
}

func getMemoryInfo(spec *specs.Spec) (*int64, *int64, *int64, *uint64, *bool) {
	if spec.Linux.Resources == nil {
		return nil, nil, nil, nil, nil
	}
	memory := spec.Linux.Resources.Memory
	if memory == nil {
		return nil, nil, nil, nil, nil
	}
	return memory.Kernel, memory.Reservation, memory.Swap, memory.Swappiness, memory.DisableOOMKiller
}

func getPidsInfo(spec *specs.Spec) *int64 {
	if spec.Linux.Resources == nil {
		return nil
	}
	pids := spec.Linux.Resources.Pids
	if pids == nil {
		return nil
	}
	return &pids.Limit
}

func getCgroup(spec *specs.Spec) string {
	cgroup := "host"
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.CgroupNamespace && ns.Path != "" {
			cgroup = "container"
		}
	}
	return cgroup
}
