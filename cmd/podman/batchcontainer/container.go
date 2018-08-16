package batchcontainer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PsOptions describes the struct being formed for ps
type PsOptions struct {
	All       bool
	Filter    string
	Format    string
	Last      int
	Latest    bool
	NoTrunc   bool
	Pod       bool
	Quiet     bool
	Size      bool
	Sort      string
	Label     string
	Namespace bool
}

// BatchContainerStruct is the return obkect from BatchContainer and contains
// container related information
type BatchContainerStruct struct {
	ConConfig   *libpod.ContainerConfig
	ConState    libpod.ContainerStatus
	ExitCode    int32
	Exited      bool
	Pid         int
	StartedTime time.Time
	ExitedTime  time.Time
	Size        *ContainerSize
}

// Namespace describes output for ps namespace
type Namespace struct {
	PID    string `json:"pid,omitempty"`
	Cgroup string `json:"cgroup,omitempty"`
	IPC    string `json:"ipc,omitempty"`
	MNT    string `json:"mnt,omitempty"`
	NET    string `json:"net,omitempty"`
	PIDNS  string `json:"pidns,omitempty"`
	User   string `json:"user,omitempty"`
	UTS    string `json:"uts,omitempty"`
}

// ContainerSize holds the size of the container's root filesystem and top
// read-write layer
type ContainerSize struct {
	RootFsSize int64 `json:"rootFsSize"`
	RwSize     int64 `json:"rwSize"`
}

// BatchContainer is used in ps to reduce performance hits by "batching"
// locks.
func BatchContainerOp(ctr *libpod.Container, opts PsOptions) (BatchContainerStruct, error) {
	var (
		conConfig   *libpod.ContainerConfig
		conState    libpod.ContainerStatus
		err         error
		exitCode    int32
		exited      bool
		pid         int
		size        *ContainerSize
		startedTime time.Time
		exitedTime  time.Time
	)

	batchErr := ctr.Batch(func(c *libpod.Container) error {
		conConfig = c.Config()
		conState, err = c.State()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container state")
		}

		exitCode, exited, err = c.ExitCode()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container exit code")
		}
		startedTime, err = c.StartedTime()
		if err != nil {
			logrus.Errorf("error getting started time for %q: %v", c.ID(), err)
		}
		exitedTime, err = c.FinishedTime()
		if err != nil {
			logrus.Errorf("error getting exited time for %q: %v", c.ID(), err)
		}

		if !opts.Size && !opts.Namespace {
			return nil
		}

		if opts.Namespace {
			pid, err = c.PID()
			if err != nil {
				return errors.Wrapf(err, "unable to obtain container pid")
			}
		}
		if opts.Size {
			size = new(ContainerSize)

			rootFsSize, err := c.RootFsSize()
			if err != nil {
				logrus.Errorf("error getting root fs size for %q: %v", c.ID(), err)
			}

			rwSize, err := c.RWSize()
			if err != nil {
				logrus.Errorf("error getting rw size for %q: %v", c.ID(), err)
			}

			size.RootFsSize = rootFsSize
			size.RwSize = rwSize
		}
		return nil
	})
	if batchErr != nil {
		return BatchContainerStruct{}, batchErr
	}
	return BatchContainerStruct{
		ConConfig:   conConfig,
		ConState:    conState,
		ExitCode:    exitCode,
		Exited:      exited,
		Pid:         pid,
		StartedTime: startedTime,
		ExitedTime:  exitedTime,
		Size:        size,
	}, nil
}

// GetNamespaces returns a populated namespace struct
func GetNamespaces(pid int) *Namespace {
	ctrPID := strconv.Itoa(pid)
	cgroup, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "cgroup"))
	ipc, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "ipc"))
	mnt, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "mnt"))
	net, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "net"))
	pidns, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "pid"))
	user, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "user"))
	uts, _ := getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "uts"))

	return &Namespace{
		PID:    ctrPID,
		Cgroup: cgroup,
		IPC:    ipc,
		MNT:    mnt,
		NET:    net,
		PIDNS:  pidns,
		User:   user,
		UTS:    uts,
	}
}

func getNamespaceInfo(path string) (string, error) {
	val, err := os.Readlink(path)
	if err != nil {
		return "", errors.Wrapf(err, "error getting info from %q", path)
	}
	return getStrFromSquareBrackets(val), nil
}

// getStrFromSquareBrackets gets the string inside [] from a string
func getStrFromSquareBrackets(cmd string) string {
	reg, err := regexp.Compile(".*\\[|\\].*")
	if err != nil {
		return ""
	}
	arr := strings.Split(reg.ReplaceAllLiteralString(cmd, ""), ",")
	return strings.Join(arr, ",")
}

// GetCtrInspectInfo takes container inspect data and collects all its info into a ContainerData
// structure for inspection related methods
func GetCtrInspectInfo(ctr *libpod.Container, ctrInspectData *inspect.ContainerInspectData) (*inspect.ContainerData, error) {
	config := ctr.Config()
	spec := config.Spec

	cpus, mems, period, quota, realtimePeriod, realtimeRuntime, shares := getCPUInfo(spec)
	blkioWeight, blkioWeightDevice, blkioReadBps, blkioWriteBps, blkioReadIOPS, blkioeWriteIOPS := getBLKIOInfo(spec)
	memKernel, memReservation, memSwap, memSwappiness, memDisableOOMKiller := getMemoryInfo(spec)
	pidsLimit := getPidsInfo(spec)
	cgroup := getCgroup(spec)

	var createArtifact cc.CreateConfig
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
			UsernsMode:           string(createArtifact.UsernsMode),
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
			Tmpfs:                createArtifact.Tmpfs,
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
