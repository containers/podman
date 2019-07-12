// +build varlink

package varlinkapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/logs"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/pkg/varlinkapi/virtwriter"
	"github.com/containers/libpod/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

// ListContainers ...
func (i *LibpodAPI) ListContainers(call iopodman.VarlinkCall) error {
	var (
		listContainers []iopodman.Container
	)

	containers, err := i.Runtime.GetAllContainers()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	opts := shared.PsOptions{
		Namespace: true,
		Size:      true,
	}
	for _, ctr := range containers {
		batchInfo, err := shared.BatchContainerOp(ctr, opts)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}

		listContainers = append(listContainers, makeListContainer(ctr.ID(), batchInfo))
	}
	return call.ReplyListContainers(listContainers)
}

func (i *LibpodAPI) Ps(call iopodman.VarlinkCall, opts iopodman.PsOpts) error {
	var (
		containers []iopodman.PsContainer
	)
	maxWorkers := shared.Parallelize("ps")
	psOpts := makePsOpts(opts)
	filters := []string{}
	if opts.Filters != nil {
		filters = *opts.Filters
	}
	psContainerOutputs, err := shared.GetPsContainerOutput(i.Runtime, psOpts, filters, maxWorkers)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	for _, ctr := range psContainerOutputs {
		container := iopodman.PsContainer{
			Id:        ctr.ID,
			Image:     ctr.Image,
			Command:   ctr.Command,
			Created:   ctr.Created,
			Ports:     ctr.Ports,
			Names:     ctr.Names,
			IsInfra:   ctr.IsInfra,
			Status:    ctr.Status,
			State:     ctr.State.String(),
			PidNum:    int64(ctr.Pid),
			Pod:       ctr.Pod,
			CreatedAt: ctr.CreatedAt.Format(time.RFC3339Nano),
			ExitedAt:  ctr.ExitedAt.Format(time.RFC3339Nano),
			StartedAt: ctr.StartedAt.Format(time.RFC3339Nano),
			Labels:    ctr.Labels,
			NsPid:     ctr.PID,
			Cgroup:    ctr.Cgroup,
			Ipc:       ctr.Cgroup,
			Mnt:       ctr.MNT,
			Net:       ctr.NET,
			PidNs:     ctr.PIDNS,
			User:      ctr.User,
			Uts:       ctr.UTS,
			Mounts:    ctr.Mounts,
		}
		if ctr.Size != nil {
			container.RootFsSize = ctr.Size.RootFsSize
			container.RwSize = ctr.Size.RwSize
		}
		containers = append(containers, container)
	}
	return call.ReplyPs(containers)
}

// GetContainer ...
func (i *LibpodAPI) GetContainer(call iopodman.VarlinkCall, id string) error {
	ctr, err := i.Runtime.LookupContainer(id)
	if err != nil {
		return call.ReplyContainerNotFound(id, err.Error())
	}
	opts := shared.PsOptions{
		Namespace: true,
		Size:      true,
	}
	batchInfo, err := shared.BatchContainerOp(ctr, opts)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyGetContainer(makeListContainer(ctr.ID(), batchInfo))
}

// GetContainersByContext returns a slice of container ids based on all, latest, or a list
func (i *LibpodAPI) GetContainersByContext(call iopodman.VarlinkCall, all, latest bool, input []string) error {
	var ids []string

	ctrs, err := shortcuts.GetContainersByContext(all, latest, input, i.Runtime)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			return call.ReplyContainerNotFound("", err.Error())
		}
		return call.ReplyErrorOccurred(err.Error())
	}

	for _, c := range ctrs {
		ids = append(ids, c.ID())
	}
	return call.ReplyGetContainersByContext(ids)
}

// GetContainersByStatus returns a slice of containers filtered by a libpod status
func (i *LibpodAPI) GetContainersByStatus(call iopodman.VarlinkCall, statuses []string) error {
	var (
		filterFuncs []libpod.ContainerFilter
		containers  []iopodman.Container
	)
	for _, status := range statuses {
		lpstatus, err := define.StringToContainerStatus(status)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == lpstatus
		})
	}
	filteredContainers, err := i.Runtime.GetContainers(filterFuncs...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	opts := shared.PsOptions{Size: true, Namespace: true}
	for _, ctr := range filteredContainers {
		batchInfo, err := shared.BatchContainerOp(ctr, opts)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		containers = append(containers, makeListContainer(ctr.ID(), batchInfo))
	}
	return call.ReplyGetContainersByStatus(containers)
}

// InspectContainer ...
func (i *LibpodAPI) InspectContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	data, err := ctr.Inspect(true)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	b, err := json.Marshal(data)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to serialize"))
	}
	return call.ReplyInspectContainer(string(b))
}

// ListContainerProcesses ...
func (i *LibpodAPI) ListContainerProcesses(call iopodman.VarlinkCall, name string, opts []string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	containerState, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if containerState != define.ContainerStateRunning {
		return call.ReplyErrorOccurred(fmt.Sprintf("container %s is not running", name))
	}
	var psArgs []string
	psOpts := []string{"user", "pid", "ppid", "pcpu", "etime", "tty", "time", "comm"}
	if len(opts) > 1 {
		psOpts = opts
	}
	psArgs = append(psArgs, psOpts...)
	psOutput, err := ctr.GetContainerPidInformation(psArgs)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyListContainerProcesses(psOutput)
}

// GetContainerLogs ...
func (i *LibpodAPI) GetContainerLogs(call iopodman.VarlinkCall, name string) error {
	var logs []string
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	logPath := ctr.LogPath()

	containerState, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if _, err := os.Stat(logPath); err != nil {
		if containerState == define.ContainerStateConfigured {
			return call.ReplyGetContainerLogs(logs)
		}
	}
	file, err := os.Open(logPath)
	if err != nil {
		return errors.Wrapf(err, "unable to read container log file")
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	if call.WantsMore() {
		call.Continues = true
	}
	for {
		line, err := reader.ReadString('\n')
		// We've read the entire file
		if err == io.EOF {
			if !call.WantsMore() {
				// If this is a non-following log request, we return what we have
				break
			} else {
				// If we want to follow, return what we have, wipe the slice, and make
				// sure the container is still running before iterating.
				call.ReplyGetContainerLogs(logs)
				logs = []string{}
				time.Sleep(1 * time.Second)
				state, err := ctr.State()
				if err != nil {
					return call.ReplyErrorOccurred(err.Error())
				}
				if state != define.ContainerStateRunning && state != define.ContainerStatePaused {
					return call.ReplyErrorOccurred(fmt.Sprintf("%s is no longer running", ctr.ID()))
				}

			}
		} else if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		} else {
			logs = append(logs, line)
		}
	}

	call.Continues = false

	return call.ReplyGetContainerLogs(logs)
}

// ListContainerChanges ...
func (i *LibpodAPI) ListContainerChanges(call iopodman.VarlinkCall, name string) error {
	changes, err := i.Runtime.GetDiff("", name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	result := iopodman.ContainerChanges{}
	for _, change := range changes {
		switch change.Kind {
		case archive.ChangeModify:
			result.Changed = append(result.Changed, change.Path)
		case archive.ChangeDelete:
			result.Deleted = append(result.Deleted, change.Path)
		case archive.ChangeAdd:
			result.Added = append(result.Added, change.Path)
		}
	}
	return call.ReplyListContainerChanges(result)
}

// ExportContainer ...
func (i *LibpodAPI) ExportContainer(call iopodman.VarlinkCall, name, outPath string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	outputFile, err := ioutil.TempFile("", "varlink_recv")
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	defer outputFile.Close()
	if outPath == "" {
		outPath = outputFile.Name()
	}
	if err := ctr.Export(outPath); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyExportContainer(outPath)

}

// GetContainerStats ...
func (i *LibpodAPI) GetContainerStats(call iopodman.VarlinkCall, name string) error {
	if rootless.IsRootless() {
		cgroupv2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		if !cgroupv2 {
			return call.ReplyErrRequiresCgroupsV2ForRootless("rootless containers cannot report container stats")
		}
	}
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	containerStats, err := ctr.GetContainerStats(&libpod.ContainerStats{})
	if err != nil {
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			return call.ReplyNoContainerRunning()
		}
		return call.ReplyErrorOccurred(err.Error())
	}
	cs := iopodman.ContainerStats{
		Id:           ctr.ID(),
		Name:         ctr.Name(),
		Cpu:          containerStats.CPU,
		Cpu_nano:     int64(containerStats.CPUNano),
		System_nano:  int64(containerStats.SystemNano),
		Mem_usage:    int64(containerStats.MemUsage),
		Mem_limit:    int64(containerStats.MemLimit),
		Mem_perc:     containerStats.MemPerc,
		Net_input:    int64(containerStats.NetInput),
		Net_output:   int64(containerStats.NetOutput),
		Block_input:  int64(containerStats.BlockInput),
		Block_output: int64(containerStats.BlockOutput),
		Pids:         int64(containerStats.PIDs),
	}
	return call.ReplyGetContainerStats(cs)
}

// StartContainer ...
func (i *LibpodAPI) StartContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	state, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if state == define.ContainerStateRunning || state == define.ContainerStatePaused {
		return call.ReplyErrorOccurred("container is already running or paused")
	}
	recursive := false
	if ctr.PodID() != "" {
		recursive = true
	}
	if err := ctr.Start(getContext(), recursive); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyStartContainer(ctr.ID())
}

// InitContainer initializes the container given by Varlink.
func (i *LibpodAPI) InitContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.Init(getContext()); err != nil {
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			return call.ReplyInvalidState(ctr.ID(), err.Error())
		}
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyInitContainer(ctr.ID())
}

// StopContainer ...
func (i *LibpodAPI) StopContainer(call iopodman.VarlinkCall, name string, timeout int64) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.StopWithTimeout(uint(timeout)); err != nil {
		if errors.Cause(err) == define.ErrCtrStopped {
			return call.ReplyErrCtrStopped(ctr.ID())
		}
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			return call.ReplyInvalidState(ctr.ID(), err.Error())
		}
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyStopContainer(ctr.ID())
}

// RestartContainer ...
func (i *LibpodAPI) RestartContainer(call iopodman.VarlinkCall, name string, timeout int64) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.RestartWithTimeout(getContext(), uint(timeout)); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRestartContainer(ctr.ID())
}

// ContainerExists looks in local storage for the existence of a container
func (i *LibpodAPI) ContainerExists(call iopodman.VarlinkCall, name string) error {
	_, err := i.Runtime.LookupContainer(name)
	if errors.Cause(err) == define.ErrNoSuchCtr {
		return call.ReplyContainerExists(1)
	}
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyContainerExists(0)
}

// KillContainer kills a running container.  If you want to use the default SIGTERM signal, just send a -1
// for the signal arg.
func (i *LibpodAPI) KillContainer(call iopodman.VarlinkCall, name string, signal int64) error {
	killSignal := uint(syscall.SIGTERM)
	if signal != -1 {
		killSignal = uint(signal)
	}
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.Kill(killSignal); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyKillContainer(ctr.ID())
}

// PauseContainer ...
func (i *LibpodAPI) PauseContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.Pause(); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyPauseContainer(ctr.ID())
}

// UnpauseContainer ...
func (i *LibpodAPI) UnpauseContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.Unpause(); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyUnpauseContainer(ctr.ID())
}

// WaitContainer ...
func (i *LibpodAPI) WaitContainer(call iopodman.VarlinkCall, name string, interval int64) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	exitCode, err := ctr.WaitWithInterval(time.Duration(interval))
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyWaitContainer(int64(exitCode))
}

// RemoveContainer ...
func (i *LibpodAPI) RemoveContainer(call iopodman.VarlinkCall, name string, force bool, removeVolumes bool) error {
	ctx := getContext()
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := i.Runtime.RemoveContainer(ctx, ctr, force, removeVolumes); err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			return call.ReplyContainerExists(1)
		}
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			return call.ReplyInvalidState(ctr.ID(), err.Error())
		}
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRemoveContainer(ctr.ID())
}

// EvictContainer ...
func (i *LibpodAPI) EvictContainer(call iopodman.VarlinkCall, name string, removeVolumes bool) error {
	ctx := getContext()
	id, err := i.Runtime.EvictContainer(ctx, name, removeVolumes)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyEvictContainer(id)
}

// DeleteStoppedContainers ...
func (i *LibpodAPI) DeleteStoppedContainers(call iopodman.VarlinkCall) error {
	ctx := getContext()
	var deletedContainers []string
	containers, err := i.Runtime.GetAllContainers()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for _, ctr := range containers {
		state, err := ctr.State()
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		if state != define.ContainerStateRunning {
			if err := i.Runtime.RemoveContainer(ctx, ctr, false, false); err != nil {
				return call.ReplyErrorOccurred(err.Error())
			}
			deletedContainers = append(deletedContainers, ctr.ID())
		}
	}
	return call.ReplyDeleteStoppedContainers(deletedContainers)
}

// GetAttachSockets ...
func (i *LibpodAPI) GetAttachSockets(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}

	status, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	// If the container hasn't been run, we need to run init
	// so the conmon sockets get created.
	if status == define.ContainerStateConfigured || status == define.ContainerStateStopped {
		if err := ctr.Init(getContext()); err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}

	sockPath, err := ctr.AttachSocketPath()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	s := iopodman.Sockets{
		Container_id:   ctr.ID(),
		Io_socket:      sockPath,
		Control_socket: ctr.ControlSocketPath(),
	}
	return call.ReplyGetAttachSockets(s)
}

// ContainerCheckpoint ...
func (i *LibpodAPI) ContainerCheckpoint(call iopodman.VarlinkCall, name string, keep, leaveRunning, tcpEstablished bool) error {
	ctx := getContext()
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}

	options := libpod.ContainerCheckpointOptions{
		Keep:           keep,
		TCPEstablished: tcpEstablished,
		KeepRunning:    leaveRunning,
	}
	if err := ctr.Checkpoint(ctx, options); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyContainerCheckpoint(ctr.ID())
}

// ContainerRestore ...
func (i *LibpodAPI) ContainerRestore(call iopodman.VarlinkCall, name string, keep, tcpEstablished bool) error {
	ctx := getContext()
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}

	options := libpod.ContainerCheckpointOptions{
		Keep:           keep,
		TCPEstablished: tcpEstablished,
	}
	if err := ctr.Restore(ctx, options); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyContainerRestore(ctr.ID())
}

// ContainerConfig returns just the container.config struct
func (i *LibpodAPI) ContainerConfig(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	config := ctr.Config()
	b, err := json.Marshal(config)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize container config")
	}
	return call.ReplyContainerConfig(string(b))
}

// ContainerArtifacts returns an untouched container's artifact in string format
func (i *LibpodAPI) ContainerArtifacts(call iopodman.VarlinkCall, name, artifactName string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	artifacts, err := ctr.GetArtifact(artifactName)
	if err != nil {
		return call.ReplyErrorOccurred("unable to get container artifacts")
	}
	b, err := json.Marshal(artifacts)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize container artifacts")
	}
	return call.ReplyContainerArtifacts(string(b))
}

// ContainerInspectData returns the inspect data of a container in string format
func (i *LibpodAPI) ContainerInspectData(call iopodman.VarlinkCall, name string, size bool) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	data, err := ctr.Inspect(size)
	if err != nil {
		return call.ReplyErrorOccurred("unable to inspect container")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize container inspect data")
	}
	return call.ReplyContainerInspectData(string(b))

}

// ContainerStateData returns a container's state data in string format
func (i *LibpodAPI) ContainerStateData(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	data, err := ctr.ContainerState()
	if err != nil {
		return call.ReplyErrorOccurred("unable to obtain container state")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize container inspect data")
	}
	return call.ReplyContainerStateData(string(b))
}

// GetContainerStatsWithHistory is a varlink endpoint that returns container stats based on current and
// previous statistics
func (i *LibpodAPI) GetContainerStatsWithHistory(call iopodman.VarlinkCall, prevStats iopodman.ContainerStats) error {
	con, err := i.Runtime.LookupContainer(prevStats.Id)
	if err != nil {
		return call.ReplyContainerNotFound(prevStats.Id, err.Error())
	}
	previousStats := ContainerStatsToLibpodContainerStats(prevStats)
	stats, err := con.GetContainerStats(&previousStats)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	cStats := iopodman.ContainerStats{
		Id:           stats.ContainerID,
		Name:         stats.Name,
		Cpu:          stats.CPU,
		Cpu_nano:     int64(stats.CPUNano),
		System_nano:  int64(stats.SystemNano),
		Mem_usage:    int64(stats.MemUsage),
		Mem_limit:    int64(stats.MemLimit),
		Mem_perc:     stats.MemPerc,
		Net_input:    int64(stats.NetInput),
		Net_output:   int64(stats.NetOutput),
		Block_input:  int64(stats.BlockInput),
		Block_output: int64(stats.BlockOutput),
		Pids:         int64(stats.PIDs),
	}
	return call.ReplyGetContainerStatsWithHistory(cStats)
}

// Spec ...
func (i *LibpodAPI) Spec(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	spec := ctr.Spec()
	b, err := json.Marshal(spec)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplySpec(string(b))
}

// GetContainersLogs is the varlink endpoint to obtain one or more container logs
func (i *LibpodAPI) GetContainersLogs(call iopodman.VarlinkCall, names []string, follow, latest bool, since string, tail int64, timestamps bool) error {
	var wg sync.WaitGroup
	if call.WantsMore() {
		call.Continues = true
	}
	sinceTime, err := time.Parse(time.RFC3339Nano, since)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	options := logs.LogOptions{
		Follow:     follow,
		Since:      sinceTime,
		Tail:       uint64(tail),
		Timestamps: timestamps,
	}

	options.WaitGroup = &wg
	if len(names) > 1 {
		options.Multi = true
	}
	logChannel := make(chan *logs.LogLine, int(tail)*len(names)+1)
	containers, err := shortcuts.GetContainersByContext(false, latest, names, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := i.Runtime.Log(containers, &options, logChannel); err != nil {
		return err
	}
	go func() {
		wg.Wait()
		close(logChannel)
	}()
	for line := range logChannel {
		call.ReplyGetContainersLogs(newPodmanLogLine(line))
		if !call.Continues {
			break
		}

	}
	return call.ReplyGetContainersLogs(iopodman.LogLine{})
}

func newPodmanLogLine(line *logs.LogLine) iopodman.LogLine {
	return iopodman.LogLine{
		Device:       line.Device,
		ParseLogType: line.ParseLogType,
		Time:         line.Time.Format(time.RFC3339Nano),
		Msg:          line.Msg,
		Cid:          line.CID,
	}
}

// Top displays information about a container's running processes
func (i *LibpodAPI) Top(call iopodman.VarlinkCall, nameOrID string, descriptors []string) error {
	ctr, err := i.Runtime.LookupContainer(nameOrID)
	if err != nil {
		return call.ReplyContainerNotFound(ctr.ID(), err.Error())
	}
	topInfo, err := ctr.Top(descriptors)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyTop(topInfo)
}

// ExecContainer is the varlink endpoint to execute a command in a container
func (i *LibpodAPI) ExecContainer(call iopodman.VarlinkCall, opts iopodman.ExecOpts) error {
	if !call.WantsUpgrade() {
		return call.ReplyErrorOccurred("client must use upgraded connection to exec")
	}

	ctr, err := i.Runtime.LookupContainer(opts.Name)
	if err != nil {
		return call.ReplyContainerNotFound(opts.Name, err.Error())
	}

	state, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(
			fmt.Sprintf("exec failed to obtain container %s state: %s", ctr.ID(), err.Error()))
	}

	if state != define.ContainerStateRunning {
		return call.ReplyErrorOccurred(
			fmt.Sprintf("exec requires a running container, %s is %s", ctr.ID(), state.String()))
	}

	// ACK the client upgrade request
	call.ReplyExecContainer()

	envs := make(map[string]string)
	if opts.Env != nil {
		// HACK: The Varlink API uses the old []string format for env,
		// storage as "k=v". Split on the = and turn into the new map
		// format.
		for _, env := range *opts.Env {
			splitEnv := strings.SplitN(env, "=", 2)
			if len(splitEnv) == 1 {
				logrus.Errorf("Got badly-formatted environment variable %q in exec", env)
				continue
			}
			envs[splitEnv[0]] = splitEnv[1]
		}
	}

	var user string
	if opts.User != nil {
		user = *opts.User
	}

	var workDir string
	if opts.Workdir != nil {
		workDir = *opts.Workdir
	}

	var detachKeys string
	if opts.DetachKeys != nil {
		detachKeys = *opts.DetachKeys
	}

	resizeChan := make(chan remotecommand.TerminalSize)

	reader, writer, _, pipeWriter, streams := setupStreams(call)

	type ExitCodeError struct {
		ExitCode uint32
		Error    error
	}
	ecErrChan := make(chan ExitCodeError, 1)

	go func() {
		if err := virtwriter.Reader(reader, nil, nil, pipeWriter, resizeChan, nil); err != nil {
			ecErrChan <- ExitCodeError{
				define.ExecErrorCodeGeneric,
				err,
			}
		}
	}()

	go func() {
		ec, err := ctr.Exec(opts.Tty, opts.Privileged, envs, opts.Cmd, user, workDir, streams, 0, resizeChan, detachKeys)
		if err != nil {
			logrus.Errorf(err.Error())
		}
		ecErrChan <- ExitCodeError{
			uint32(ec),
			err,
		}
	}()

	ecErr := <-ecErrChan

	exitCode := define.TranslateExecErrorToExitCode(int(ecErr.ExitCode), ecErr.Error)

	if err = virtwriter.HangUp(writer, uint32(exitCode)); err != nil {
		logrus.Errorf("ExecContainer failed to HANG-UP on %s: %s", ctr.ID(), err.Error())
	}

	if err := call.Writer.Flush(); err != nil {
		logrus.Errorf("Exec Container err: %s", err.Error())
	}

	return ecErr.Error
}

//HealthCheckRun executes defined container's healthcheck command and returns the container's health status.
func (i *LibpodAPI) HealthCheckRun(call iopodman.VarlinkCall, nameOrID string) error {
	hcStatus, err := i.Runtime.HealthCheck(nameOrID)
	if err != nil && hcStatus != libpod.HealthCheckFailure {
		return call.ReplyErrorOccurred(err.Error())
	}
	status := libpod.HealthCheckUnhealthy
	if hcStatus == libpod.HealthCheckSuccess {
		status = libpod.HealthCheckHealthy
	}
	return call.ReplyHealthCheckRun(status)
}

// CopyTo copies a file from local or reomte to a container
func (i *LibpodAPI) CopyTo(call iopodman.VarlinkCall, src string, srcPath string, srcfMode int64, destPath string, extract bool, pause bool, ctrNameorId string) error {
	ctr, err := i.Runtime.LookupContainer(ctrNameorId)

	if err != nil {
		return errors.Wrapf(err, "error getting container %s", ctrNameorId)
	}
	mountPoint, err := ctr.Mount()
	if err != nil {
		logrus.Debugf("mount container error %q", err)
		return errors.Wrapf(err, "error getting mount point of container %s", ctrNameorId)
	}
	defer func() {
		if err := ctr.Unmount(false); err != nil {
			logrus.Errorf("unable to umount container '%s': %q", ctr.ID(), err)
		}
	}()

	// We can't pause rootless containers.
	if pause && rootless.IsRootless() {
		state, err := ctr.State()
		if err != nil {
			return err
		}
		if state == define.ContainerStateRunning {
			return errors.Errorf("cannot copy into running rootless container with pause set - pass --pause=false to force copying")
		}
	}
	if pause && !rootless.IsRootless() {
		if err := ctr.Pause(); err != nil {
			// An invalid state error is fine.
			// The container isn't running or is already paused.
			// TODO: We can potentially start the container while
			// the copy is running, which still allows a race where
			// malicious code could mess with the symlink.
			if errors.Cause(err) != define.ErrCtrStateInvalid {
				return err
			}
		} else if err == nil {
			// Only add the defer if we actually paused
			defer func() {
				if err := ctr.Unpause(); err != nil {
					logrus.Errorf("Error unpausing container after copying: %v", err)
				}
			}()
		}
	}

	user, err := util.GetUser(mountPoint, ctr.User())
	if err != nil {
		return err
	}
	idMappingOpts, err := ctr.IDMappings()
	if err != nil {
		return errors.Wrapf(err, "error getting IDMappingOptions")
	}
	destOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}
	copyFileWithTar := chrootarchive.CopyFileWithTarAndChown(&destOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	untarPath := chrootarchive.UntarPathAndChown(&destOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	copyWithTar := chrootarchive.CopyWithTarAndChown(&destOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)

	mode := os.FileMode(srcfMode)
	srcIsDir := mode.IsDir()

	cleanDestPath := filepath.Clean(destPath)

	if filepath.IsAbs(cleanDestPath) {
		cleanedPath, err := securejoin.SecureJoin(mountPoint, cleanDestPath)
		if err != nil {
			return err
		}
		cleanDestPath = cleanedPath

	} else {
		cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), cleanDestPath))
		if err != nil {
			return err
		}
		cleanDestPath = cleanedPath
	}

	cleanFilepath := filepath.Clean(srcPath)
	if src == "local" {
		destPath, err = CopyToLocal(cleanFilepath, cleanDestPath, srcPath, destPath, extract, copyFileWithTar, untarPath, copyWithTar)
		if err != nil {
			return err
		}
		return call.ReplyCopyTo(destPath)
	}

	// parse the combined filepath
	// /tmp/varlink_send/filename/.
	filename := filepath.Base(cleanFilepath)
	tempFile := filepath.Clean(filepath.Dir(cleanFilepath))

	destPath, err = util.CopyRemote(filename, srcPath, tempFile, destPath, cleanDestPath, srcIsDir, &destOwner, untarPath, copyFileWithTar, extract)
	if err != nil {
		return errors.Wrapf(err, "error copying %q to %q", tempFile, cleanDestPath)
	}

	err = call.ReplyCopyTo(destPath)
	if err != nil {
		logrus.Debug("error ReplyCopyTo", err)
	}
	return err
}

// CopyToLocal copy file from cleanSrcPath to cleanDestPath when the paths are from the same host
func CopyToLocal(cleanSrcPath string, cleanDestPath string, srcPath string, destPath string, extract bool, copyFileWithTar, untarPath, copyWithTar func(src, dest string) error) (string, error) {
	cleanSrcPath, err := filepath.EvalSymlinks(cleanSrcPath)
	if err != nil {
		return "", errors.Wrapf(err, "error evaluating symlinks %q", cleanSrcPath)
	}
	cleanSrcPath, srcfi, err := util.GetPathInfo(cleanSrcPath)
	if err != nil {
		return "", err
	}
	destdir := cleanDestPath
	if !srcfi.IsDir() {
		destdir = filepath.Dir(cleanDestPath)
	}
	_, err = os.Stat(destdir)
	if err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error checking directory %q", destdir)
	}
	destDirIsExist := (err == nil)
	if err = os.MkdirAll(destdir, 0755); err != nil {
		return "", errors.Wrapf(err, "error creating directory %q", destdir)
	}
	if srcfi.IsDir() {
		logrus.Debugf("copying %q to %q", cleanSrcPath+string(os.PathSeparator)+"*", cleanDestPath+string(os.PathSeparator)+"*")
		destPath = cleanDestPath
		if destDirIsExist && !strings.HasSuffix(srcPath, fmt.Sprintf("%s.", string(os.PathSeparator))) {
			destPath = filepath.Join(cleanDestPath, filepath.Base(cleanSrcPath))
		}
		if err = copyWithTar(cleanSrcPath, destPath); err != nil {
			logrus.Errorf("error copying %q to %q, error %q", cleanSrcPath, destPath, err)
			return "", errors.Wrapf(err, "error copying %q to %q", cleanSrcPath, destPath)
		}
		return destPath, nil
	}
	if archive.IsArchivePath(cleanSrcPath) && extract {
		if err := untarPath(cleanSrcPath, cleanDestPath); err != nil {
			return "", errors.Wrapf(err, "error copying %q to %q", cleanSrcPath, cleanDestPath)
		}
		return cleanDestPath, nil
	}
	// copy a file or tarball
	destfi, err := os.Stat(cleanDestPath)
	if err != nil {
		if !os.IsNotExist(err) || strings.HasSuffix(destPath, string(os.PathSeparator)) {
			logrus.Debugf("error copying %s to remote %s : %q", cleanSrcPath, destPath, err)
			return "", errors.Wrapf(err, "failed to get stat of dest path %s", cleanDestPath)
		}
	}
	destPath = cleanDestPath
	if destfi != nil && destfi.IsDir() {
		destPath = filepath.Join(cleanDestPath, filepath.Base(cleanSrcPath))
	}
	err = copyFileWithTar(cleanSrcPath, destPath)
	if err != nil {
		logrus.Errorf("error copying file from %s to %s", cleanSrcPath, destPath)
		return "", errors.Wrapf(err, "error copying to %s", destPath)
	}
	return destPath, nil
}

// CopyFrom copies a file from local container to local file system
// returns the helper string containers source path used by CopyFromContainer if copy a file from remote container to local file system using podman remote
func (i *LibpodAPI) CopyFrom(call iopodman.VarlinkCall, dest string, srcPath string, srcfMode int64, destPath string, extract bool, pause bool, ctrNameorId string) error {
	ctr, err := i.Runtime.LookupContainer(ctrNameorId)

	if err != nil {
		return errors.Wrapf(err, "error getting container %s", ctrNameorId)
	}

	mountPoint, err := ctr.Mount()
	if err != nil {
		logrus.Errorf("unable to mount container '%s': %q", ctr.ID(), err)
		return errors.Wrapf(err, "error getting mount point of container %s", ctrNameorId)
	}
	defer func() {
		if err := ctr.Unmount(false); err != nil {
			logrus.Errorf("unable to umount container '%s': %q", ctr.ID(), err)
		}
	}()

	// We can't pause rootless containers.
	if pause && rootless.IsRootless() {
		state, err := ctr.State()
		if err != nil {
			return err
		}
		if state == define.ContainerStateRunning {
			return errors.Errorf("cannot copy into running rootless container with pause set - pass --pause=false to force copying")
		}
	}
	if pause && !rootless.IsRootless() {
		if err := ctr.Pause(); err != nil {
			// An invalid state error is fine.
			// The container isn't running or is already paused.
			// TODO: We can potentially start the container while
			// the copy is running, which still allows a race where
			// malicious code could mess with the symlink.
			if errors.Cause(err) != define.ErrCtrStateInvalid {
				return err
			}
		} else if err == nil {
			// Only add the defer if we actually paused
			defer func() {
				if err := ctr.Unpause(); err != nil {
					logrus.Errorf("Error unpausing container after copying: %v", err)
				}
			}()
		}
	}

	// combines source path with mount point
	cleanSrcPath := filepath.Clean(srcPath)
	if filepath.IsAbs(cleanSrcPath) {
		cleanedPath, err := securejoin.SecureJoin(mountPoint, cleanSrcPath)
		if err != nil {
			return err
		}
		cleanSrcPath = cleanedPath
	} else {
		cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), cleanSrcPath))
		if err != nil {
			return err
		}
		cleanSrcPath = cleanedPath
	}

	user, err := util.GetUser(mountPoint, ctr.User())
	if err != nil {
		return err
	}

	idMappingOpts, err := ctr.IDMappings()
	if err != nil {
		return errors.Wrapf(err, "error getting IDMappingOptions")
	}
	containerOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}
	copyFileWithTar := chrootarchive.CopyFileWithTarAndChown(&containerOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	untarPath := chrootarchive.UntarPathAndChown(&containerOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	copyWithTar := chrootarchive.CopyWithTarAndChown(&containerOwner, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)

	if dest == "local" {
		// resolve absolute destPath
		cleanDestPath := filepath.Clean(destPath)
		if !filepath.IsAbs(cleanDestPath) {
			dir, err := os.Getwd()
			if err != nil {
				return errors.Wrapf(err, "err getting current working directory")
			}
			cleanDestPath = filepath.Join(dir, cleanDestPath)
		}

		destPath, err := CopyToLocal(cleanSrcPath, cleanDestPath, srcPath, destPath, extract, copyFileWithTar, untarPath, copyWithTar)
		if err != nil {
			return err
		}
		return call.ReplyCopyFrom(destPath)
	}

	// if dest is remote, tars up source file and returns the path of archive files
	filename := filepath.Base(cleanSrcPath)
	_, srcfi, err := util.GetPathInfo(cleanSrcPath)
	if err != nil {
		return err
	}
	// tars the source file up  and sends it to remote path tempFile
	outputFile, err := ioutil.TempFile("", "varlink_tar_send")
	if err != nil {
		return err
	}
	if err := utils.TarToFilesystem(cleanSrcPath, outputFile); err != nil {
		return err
	}
	srcfModeStr := strconv.Itoa(int(srcfi.Mode()))
	// combines the tempfile path with filename, filemode /tmp/varlink_tar_send/filename/filemode
	cleanSrcPath = filepath.Join(outputFile.Name(), filename, srcfModeStr)
	return call.ReplyCopyFrom(cleanSrcPath)
}
