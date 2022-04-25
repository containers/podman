package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/filters"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/ps"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Force         bool  `schema:"force"`
		Ignore        bool  `schema:"ignore"`
		Depend        bool  `schema:"depend"`
		Link          bool  `schema:"link"`
		Timeout       *uint `schema:"timeout"`
		DockerVolumes bool  `schema:"v"`
		LibpodVolumes bool  `schema:"volumes"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.RmOptions{
		Force:  query.Force,
		Ignore: query.Ignore,
	}
	if utils.IsLibpodRequest(r) {
		options.Volumes = query.LibpodVolumes
		options.Timeout = query.Timeout
		options.Depend = query.Depend
	} else {
		if query.Link {
			utils.Error(w, http.StatusBadRequest, utils.ErrLinkNotSupport)
			return
		}
		options.Volumes = query.DockerVolumes
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	reports, err := containerEngine.ContainerRm(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}
	if len(reports) > 0 && reports[0].Err != nil {
		err = reports[0].Err
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All   bool `schema:"all"`
		Limit int  `schema:"limit"`
		Size  bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to decode filter parameters for %s", r.URL.String()))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	filterFuncs := make([]libpod.ContainerFilter, 0, len(*filterMap))
	all := query.All || query.Limit > 0
	if len((*filterMap)) > 0 {
		for k, v := range *filterMap {
			generatedFunc, err := filters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	// Docker thinks that if status is given as an input, then we should override
	// the all setting and always deal with all containers.
	if len((*filterMap)["status"]) > 0 {
		all = true
	}
	if !all {
		runningOnly, err := filters.GenerateContainerFilterFuncs("status", []string{define.ContainerStateRunning.String()}, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, runningOnly)
	}

	containers, err := runtime.GetContainers(filterFuncs...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if _, found := r.URL.Query()["limit"]; found && query.Limit > 0 {
		// Sort the libpod containers
		sort.Sort(ps.SortCreateTime{SortContainers: containers})
		// we should perform the lopping before we start getting
		// the expensive information on containers
		if len(containers) > query.Limit {
			containers = containers[:query.Limit]
		}
	}
	list := make([]*handlers.Container, 0, len(containers))
	for _, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, query.Size)
		if err != nil {
			if errors.Cause(err) == define.ErrNoSuchCtr {
				// container was removed between the initial fetch of the list and conversion
				logrus.Debugf("Container %s removed between initial fetch and conversion, ignoring in output", ctnr.ID())
				continue
			}
			utils.InternalServerError(w, err)
			return
		}
		list = append(list, api)
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	api, err := LibpodToContainerJSON(ctnr, query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, api)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Signal string `schema:"signal"`
	}{
		Signal: "KILL",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	options := entities.KillOptions{
		Signal: query.Signal,
	}
	report, err := containerEngine.ContainerKill(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Cause(err) == define.ErrCtrStateInvalid ||
			errors.Cause(err) == define.ErrCtrStopped {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		utils.InternalServerError(w, report[0].Err)
		return
	}
	// Docker waits for the container to stop if the signal is 0 or
	// SIGKILL.
	if !utils.IsLibpodRequest(r) {
		sig, err := signal.ParseSignalNameOrNumber(query.Signal)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
			opts := entities.WaitOptions{
				Condition: []define.ContainerStatus{define.ContainerStateExited, define.ContainerStateStopped},
				Interval:  time.Millisecond * 250,
			}
			if _, err := containerEngine.ContainerWait(r.Context(), []string{name}, opts); err != nil {
				utils.Error(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/wait
	utils.WaitContainerDocker(w, r)
}

func LibpodToContainer(l *libpod.Container, sz bool) (*handlers.Container, error) {
	imageID, imageName := l.Image()

	var (
		err        error
		sizeRootFs int64
		sizeRW     int64
		state      define.ContainerStatus
		status     string
	)

	if state, err = l.State(); err != nil {
		return nil, err
	}
	stateStr := state.String()
	if stateStr == "configured" {
		stateStr = "created"
	}

	switch state {
	case define.ContainerStateConfigured, define.ContainerStateCreated:
		status = "Created"
	case define.ContainerStateStopped, define.ContainerStateExited:
		exitCode, _, err := l.ExitCode()
		if err != nil {
			return nil, err
		}
		finishedTime, err := l.FinishedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Exited (%d) %s ago", exitCode, units.HumanDuration(time.Since(finishedTime)))
	case define.ContainerStateRunning, define.ContainerStatePaused:
		startedTime, err := l.StartedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Up %s", units.HumanDuration(time.Since(startedTime)))
		if state == define.ContainerStatePaused {
			status += " (Paused)"
		}
	case define.ContainerStateRemoving:
		status = "Removal In Progress"
	case define.ContainerStateStopping:
		status = "Stopping"
	default:
		status = "Unknown"
	}

	if sz {
		if sizeRW, err = l.RWSize(); err != nil {
			return nil, err
		}
		if sizeRootFs, err = l.RootFsSize(); err != nil {
			return nil, err
		}
	}

	portMappings, err := l.PortMappings()
	if err != nil {
		return nil, err
	}

	ports := make([]types.Port, len(portMappings))
	for idx, portMapping := range portMappings {
		ports[idx] = types.Port{
			IP:          portMapping.HostIP,
			PrivatePort: uint16(portMapping.ContainerPort),
			PublicPort:  uint16(portMapping.HostPort),
			Type:        portMapping.Protocol,
		}
	}
	inspect, err := l.Inspect(false)
	if err != nil {
		return nil, err
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}
	networkSettings := types.SummaryNetworkSettings{}
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []types.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	return &handlers.Container{Container: types.Container{
		ID:         l.ID(),
		Names:      []string{fmt.Sprintf("/%s", l.Name())},
		Image:      imageName,
		ImageID:    "sha256:" + imageID,
		Command:    strings.Join(l.Command(), " "),
		Created:    l.CreatedTime().Unix(),
		Ports:      ports,
		SizeRw:     sizeRW,
		SizeRootFs: sizeRootFs,
		Labels:     l.Labels(),
		State:      stateStr,
		Status:     status,
		HostConfig: struct {
			NetworkMode string `json:",omitempty"`
		}{
			"host"},
		NetworkSettings: &networkSettings,
		Mounts:          mounts,
	},
		ContainerCreateConfig: types.ContainerCreateConfig{},
	}, nil
}

func LibpodToContainerJSON(l *libpod.Container, sz bool) (*types.ContainerJSON, error) {
	_, imageName := l.Image()
	inspect, err := l.Inspect(sz)
	if err != nil {
		return nil, err
	}
	// Docker uses UTC
	if inspect != nil && inspect.State != nil {
		inspect.State.StartedAt = inspect.State.StartedAt.UTC()
		inspect.State.FinishedAt = inspect.State.FinishedAt.UTC()
	}
	i, err := json.Marshal(inspect.State)
	if err != nil {
		return nil, err
	}
	state := types.ContainerState{}
	if err := json.Unmarshal(i, &state); err != nil {
		return nil, err
	}

	// docker considers paused to be running
	if state.Paused {
		state.Running = true
	}

	// docker calls the configured state "created"
	if state.Status == define.ContainerStateConfigured.String() {
		state.Status = define.ContainerStateCreated.String()
	}

	if l.HasHealthCheck() && state.Status != "created" {
		state.Health = &types.Health{
			Status:        inspect.State.Health.Status,
			FailingStreak: inspect.State.Health.FailingStreak,
		}

		log := inspect.State.Health.Log

		for _, item := range log {
			res := &types.HealthcheckResult{}
			s, _ := time.Parse(time.RFC3339Nano, item.Start)
			e, _ := time.Parse(time.RFC3339Nano, item.End)
			res.Start = s
			res.End = e
			res.ExitCode = item.ExitCode
			res.Output = item.Output
			state.Health.Log = append(state.Health.Log, res)
		}
	}

	formatCapabilities(inspect.HostConfig.CapDrop)
	formatCapabilities(inspect.HostConfig.CapAdd)

	h, err := json.Marshal(inspect.HostConfig)
	if err != nil {
		return nil, err
	}
	hc := container.HostConfig{}
	if err := json.Unmarshal(h, &hc); err != nil {
		return nil, err
	}

	// k8s-file == json-file
	if hc.LogConfig.Type == define.KubernetesLogging {
		hc.LogConfig.Type = define.JSONLogging
	}
	g, err := json.Marshal(inspect.GraphDriver)
	if err != nil {
		return nil, err
	}
	graphDriver := types.GraphDriverData{}
	if err := json.Unmarshal(g, &graphDriver); err != nil {
		return nil, err
	}

	cb := types.ContainerJSONBase{
		ID:              l.ID(),
		Created:         l.CreatedTime().UTC().Format(time.RFC3339Nano), // Docker uses UTC
		Path:            inspect.Path,
		Args:            inspect.Args,
		State:           &state,
		Image:           imageName,
		ResolvConfPath:  inspect.ResolvConfPath,
		HostnamePath:    inspect.HostnamePath,
		HostsPath:       inspect.HostsPath,
		LogPath:         l.LogPath(),
		Node:            nil,
		Name:            fmt.Sprintf("/%s", l.Name()),
		RestartCount:    int(inspect.RestartCount),
		Driver:          inspect.Driver,
		Platform:        "linux",
		MountLabel:      inspect.MountLabel,
		ProcessLabel:    inspect.ProcessLabel,
		AppArmorProfile: inspect.AppArmorProfile,
		ExecIDs:         inspect.ExecIDs,
		HostConfig:      &hc,
		GraphDriver:     graphDriver,
		SizeRw:          inspect.SizeRw,
		SizeRootFs:      &inspect.SizeRootFs,
	}

	// set Path and Args
	processArgs := l.Config().Spec.Process.Args
	if len(processArgs) > 0 {
		cb.Path = processArgs[0]
	}
	if len(processArgs) > 1 {
		cb.Args = processArgs[1:]
	}
	stopTimeout := int(l.StopTimeout())

	exposedPorts := make(nat.PortSet)
	for ep := range inspect.HostConfig.PortBindings {
		splitp := strings.SplitN(ep, "/", 2)
		if len(splitp) != 2 {
			return nil, errors.Errorf("PORT/PROTOCOL Format required for %q", ep)
		}
		exposedPort, err := nat.NewPort(splitp[1], splitp[0])
		if err != nil {
			return nil, err
		}
		exposedPorts[exposedPort] = struct{}{}
	}

	var healthcheck *container.HealthConfig
	if inspect.Config.Healthcheck != nil {
		healthcheck = &container.HealthConfig{
			Test:        inspect.Config.Healthcheck.Test,
			Interval:    inspect.Config.Healthcheck.Interval,
			Timeout:     inspect.Config.Healthcheck.Timeout,
			StartPeriod: inspect.Config.Healthcheck.StartPeriod,
			Retries:     inspect.Config.Healthcheck.Retries,
		}
	}

	config := container.Config{
		Hostname:        l.Hostname(),
		Domainname:      inspect.Config.DomainName,
		User:            l.User(),
		AttachStdin:     inspect.Config.AttachStdin,
		AttachStdout:    inspect.Config.AttachStdout,
		AttachStderr:    inspect.Config.AttachStderr,
		ExposedPorts:    exposedPorts,
		Tty:             inspect.Config.Tty,
		OpenStdin:       inspect.Config.OpenStdin,
		StdinOnce:       inspect.Config.StdinOnce,
		Env:             inspect.Config.Env,
		Cmd:             l.Command(),
		Healthcheck:     healthcheck,
		ArgsEscaped:     false,
		Image:           imageName,
		Volumes:         nil,
		WorkingDir:      l.WorkingDir(),
		Entrypoint:      l.Entrypoint(),
		NetworkDisabled: false,
		MacAddress:      "",
		OnBuild:         nil,
		Labels:          l.Labels(),
		StopSignal:      strconv.Itoa(int(l.StopSignal())),
		StopTimeout:     &stopTimeout,
		Shell:           nil,
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []types.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	p, err := json.Marshal(inspect.NetworkSettings.Ports)
	if err != nil {
		return nil, err
	}
	ports := nat.PortMap{}
	if err := json.Unmarshal(p, &ports); err != nil {
		return nil, err
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}

	networkSettings := types.NetworkSettings{}
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}
	// do not report null instead use an empty map
	if networkSettings.Networks == nil {
		networkSettings.Networks = map[string]*network.EndpointSettings{}
	}

	c := types.ContainerJSON{
		ContainerJSONBase: &cb,
		Mounts:            mounts,
		Config:            &config,
		NetworkSettings:   &networkSettings,
	}
	return &c, nil
}

func formatCapabilities(slice []string) {
	for i := range slice {
		slice[i] = strings.TrimPrefix(slice[i], "CAP_")
	}
}

func RenameContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	name := utils.GetName(r)
	query := struct {
		Name string `schema:"name"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if _, err := runtime.RenameContainer(r.Context(), ctr, query.Name); err != nil {
		if errors.Cause(err) == define.ErrPodExists || errors.Cause(err) == define.ErrCtrExists {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}
