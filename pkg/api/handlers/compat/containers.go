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

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/filters"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/containers/podman/v2/pkg/ps"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All     bool `schema:"all"`
		Force   bool `schema:"force"`
		Ignore  bool `schema:"ignore"`
		Link    bool `schema:"link"`
		Volumes bool `schema:"v"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.Link && !utils.IsLibpodRequest(r) {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			utils.ErrLinkNotSupport)
		return
	}

	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	options := entities.RmOptions{
		All:     query.All,
		Force:   query.Force,
		Volumes: query.Volumes,
		Ignore:  query.Ignore,
	}
	report, err := containerEngine.ContainerRm(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}
	if report[0].Err != nil {
		utils.InternalServerError(w, report[0].Err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All     bool                `schema:"all"`
		Limit   int                 `schema:"limit"`
		Size    bool                `schema:"size"`
		Filters map[string][]string `schema:"filters"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	filterFuncs := make([]libpod.ContainerFilter, 0, len(query.Filters))
	all := query.All || query.Limit > 0
	if len(query.Filters) > 0 {
		for k, v := range query.Filters {
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
	if len(query.Filters["status"]) > 0 {
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
	var list = make([]*handlers.Container, len(containers))
	for i, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, query.Size)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		list[i] = api
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Signal string `schema:"signal"`
	}{
		Signal: "KILL",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	sig, err := signal.ParseSignalNameOrNumber(query.Signal)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill Container %s, it is not running", name)))
		return
	}

	signal := uint(sig)

	err = con.Kill(signal)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "unable to kill Container %s", name))
		return
	}

	// Docker waits for the container to stop if the signal is 0 or
	// SIGKILL.
	if !utils.IsLibpodRequest(r) && (signal == 0 || syscall.Signal(signal) == syscall.SIGKILL) {
		if _, err = con.Wait(); err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to wait for Container %s", con.ID()))
			return
		}
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	var msg string
	// /{version}/containers/(name)/wait
	exitCode, err := utils.WaitContainer(w, r)
	if err != nil {
		logrus.Warnf("failed to wait on container %q: %v", mux.Vars(r)["name"], err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, handlers.ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func LibpodToContainer(l *libpod.Container, sz bool) (*handlers.Container, error) {
	imageID, imageName := l.Image()

	var (
		err        error
		sizeRootFs int64
		sizeRW     int64
		state      define.ContainerStatus
	)

	if state, err = l.State(); err != nil {
		return nil, err
	}
	stateStr := state.String()
	if stateStr == "configured" {
		stateStr = "created"
	}

	if sz {
		if sizeRW, err = l.RWSize(); err != nil {
			return nil, err
		}
		if sizeRootFs, err = l.RootFsSize(); err != nil {
			return nil, err
		}
	}

	return &handlers.Container{Container: types.Container{
		ID:         l.ID(),
		Names:      []string{fmt.Sprintf("/%s", l.Name())},
		Image:      imageName,
		ImageID:    imageID,
		Command:    strings.Join(l.Command(), " "),
		Created:    l.CreatedTime().Unix(),
		Ports:      nil,
		SizeRw:     sizeRW,
		SizeRootFs: sizeRootFs,
		Labels:     l.Labels(),
		State:      stateStr,
		Status:     "",
		HostConfig: struct {
			NetworkMode string `json:",omitempty"`
		}{
			"host"},
		NetworkSettings: nil,
		Mounts:          nil,
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
		Created:         l.CreatedTime().Format(time.RFC3339Nano),
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
		Healthcheck:     nil,
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	name := utils.GetName(r)
	query := struct {
		Name string `schema:"name"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if _, err := runtime.RenameContainer(r.Context(), ctr, query.Name); err != nil {
		if errors.Cause(err) == define.ErrPodExists || errors.Cause(err) == define.ErrCtrExists {
			utils.Error(w, "Something went wrong.", http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}
