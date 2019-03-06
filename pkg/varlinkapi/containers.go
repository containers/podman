package varlinkapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
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
		return call.ReplyErrorOccurred(err.Error())
	}

	for _, c := range ctrs {
		ids = append(ids, c.ID())
	}
	return call.ReplyGetContainersByContext(ids)
}

// InspectContainer ...
func (i *LibpodAPI) InspectContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	inspectInfo, err := ctr.Inspect(true)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	artifact, err := getArtifact(ctr)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	data, err := shared.GetCtrInspectInfo(ctr.Config(), inspectInfo, artifact)
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
	if containerState != libpod.ContainerStateRunning {
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
		if containerState == libpod.ContainerStateConfigured {
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
				if state != libpod.ContainerStateRunning && state != libpod.ContainerStatePaused {
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
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	containerStats, err := ctr.GetContainerStats(&libpod.ContainerStats{})
	if err != nil {
		if errors.Cause(err) == libpod.ErrCtrStateInvalid {
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
	if state == libpod.ContainerStateRunning || state == libpod.ContainerStatePaused {
		return call.ReplyErrorOccurred("container is already running or paused")
	}
	if err := ctr.Start(getContext(), false); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyStartContainer(ctr.ID())
}

// StopContainer ...
func (i *LibpodAPI) StopContainer(call iopodman.VarlinkCall, name string, timeout int64) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	if err := ctr.StopWithTimeout(uint(timeout)); err != nil && err != libpod.ErrCtrStopped {
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
	if errors.Cause(err) == libpod.ErrNoSuchCtr {
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
func (i *LibpodAPI) WaitContainer(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	exitCode, err := ctr.Wait()
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
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRemoveContainer(ctr.ID())

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
		if state != libpod.ContainerStateRunning {
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
	if status == libpod.ContainerStateConfigured || status == libpod.ContainerStateStopped {
		if err := ctr.Init(getContext()); err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}

	s := iopodman.Sockets{
		Container_id:   ctr.ID(),
		Io_socket:      ctr.AttachSocketPath(),
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

func getArtifact(ctr *libpod.Container) (*cc.CreateConfig, error) {
	var createArtifact cc.CreateConfig
	artifact, err := ctr.GetArtifact("create-config")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(artifact, &createArtifact); err != nil {
		return nil, err
	}
	return &createArtifact, nil
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
func (i *LibpodAPI) ContainerInspectData(call iopodman.VarlinkCall, name string) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	data, err := ctr.Inspect(true)
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

// ContainerStatsToLibpodContainerStats converts the varlink containerstats to a libpod
// container stats
func ContainerStatsToLibpodContainerStats(stats iopodman.ContainerStats) libpod.ContainerStats {
	cstats := libpod.ContainerStats{
		ContainerID: stats.Id,
		Name:        stats.Name,
		CPU:         stats.Cpu,
		CPUNano:     uint64(stats.Cpu_nano),
		SystemNano:  uint64(stats.System_nano),
		MemUsage:    uint64(stats.Mem_usage),
		MemLimit:    uint64(stats.Mem_limit),
		MemPerc:     stats.Mem_perc,
		NetInput:    uint64(stats.Net_input),
		NetOutput:   uint64(stats.Net_output),
		BlockInput:  uint64(stats.Block_input),
		BlockOutput: uint64(stats.Block_output),
		PIDs:        uint64(stats.Pids),
	}
	return cstats
}
