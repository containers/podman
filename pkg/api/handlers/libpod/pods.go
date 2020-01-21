package libpod

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/util"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PodCreate(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 internal
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		options []libpod.PodCreateOption
		err     error
	)
	labels := make(map[string]string)
	input := handlers.PodCreateConfig{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if len(input.InfraCommand) > 0 || len(input.InfraImage) > 0 {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError,
			errors.New("infra-command and infra-image are not implemented yet"))
		return
	}
	// TODO long term we should break the following out of adapter and into libpod proper
	// so that the cli and api can share the creation of a pod with the same options
	if len(input.CGroupParent) > 0 {
		options = append(options, libpod.WithPodCgroupParent(input.CGroupParent))
	}

	if len(input.Labels) > 0 {
		if err := parse.ReadKVStrings(labels, []string{}, input.Labels); err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
	}

	if len(labels) != 0 {
		options = append(options, libpod.WithPodLabels(labels))
	}

	if len(input.Name) > 0 {
		options = append(options, libpod.WithPodName(input.Name))
	}

	if len(input.Hostname) > 0 {
		options = append(options, libpod.WithPodHostname(input.Hostname))
	}

	if input.Infra {
		// TODO infra-image and infra-command are not supported in the libpod API yet.  Will fix
		// when implemented in libpod
		options = append(options, libpod.WithInfraContainer())
		sharedNamespaces := shared.DefaultKernelNamespaces
		if len(input.Share) > 0 {
			sharedNamespaces = input.Share
		}
		nsOptions, err := shared.GetNamespaceOptions(strings.Split(sharedNamespaces, ","))
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
		options = append(options, nsOptions...)
	}

	if len(input.Publish) > 0 {
		portBindings, err := shared.CreatePortBindings(input.Publish)
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	// always have containers use pod cgroups
	// User Opt out is not yet supported
	options = append(options, libpod.WithPodCgroups())

	pod, err := runtime.NewPod(r.Context(), options...)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, handlers.IDResponse{ID: pod.CgroupParent()})
}

func Pods(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 internal
	var (
		runtime        = r.Context().Value("runtime").(*libpod.Runtime)
		podInspectData []*libpod.PodInspect
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		filters []string `schema:"filters"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if len(query.filters) > 0 {
		utils.Error(w, "filters are not implemented yet", http.StatusInternalServerError, define.ErrNotImplemented)
		return
	}
	pods, err := runtime.GetAllPods()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	for _, pod := range pods {
		data, err := pod.Inspect()
		if err != nil {
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		podInspectData = append(podInspectData, data)
	}
	utils.WriteResponse(w, http.StatusOK, podInspectData)
}

func PodInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	podData, err := pod.Inspect()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, podData)
}

func PodStop(w http.ResponseWriter, r *http.Request) {
	// 200
	// 304 not modified
	// 404 no such
	// 500 internal
	var (
		stopError error
		runtime   = r.Context().Value("runtime").(*libpod.Runtime)
		decoder   = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
		timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	allContainersStopped := true
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	// TODO we need to implement a pod.State/Status in libpod internal so libpod api
	// users dont have to run through all containers.
	podContainers, err := pod.AllContainers()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	for _, con := range podContainers {
		containerState, err := con.State()
		if err != nil {
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		if containerState == define.ContainerStateRunning {
			allContainersStopped = false
			break
		}
	}
	if allContainersStopped {
		alreadyStopped := errors.Errorf("pod %s is already stopped", pod.ID())
		utils.Error(w, "Something went wrong", http.StatusNotModified, alreadyStopped)
		return
	}

	if query.timeout > 0 {
		_, stopError = pod.StopWithTimeout(r.Context(), false, query.timeout)
	} else {
		_, stopError = pod.Stop(r.Context(), false)
	}
	if stopError != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PodStart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	allContainersRunning := true
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	// TODO we need to implement a pod.State/Status in libpod internal so libpod api
	// users dont have to run through all containers.
	podContainers, err := pod.AllContainers()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	for _, con := range podContainers {
		containerState, err := con.State()
		if err != nil {
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		if containerState != define.ContainerStateRunning {
			allContainersRunning = false
			break
		}
	}
	if allContainersRunning {
		alreadyRunning := errors.Errorf("pod %s is already running", pod.ID())
		utils.Error(w, "Something went wrong", http.StatusNotModified, alreadyRunning)
		return
	}
	if _, err := pod.Start(r.Context()); err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PodDelete(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
		force bool `schema:"force"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	if err := runtime.RemovePod(r.Context(), pod, true, query.force); err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PodRestart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	_, err = pod.Restart(r.Context())
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PodPrune(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		pods    []*libpod.Pod
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
		force bool `schema:"force"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.force {
		pods, err = runtime.GetAllPods()
		if err != nil {
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
	} else {
		// TODO We need to make a libpod.PruneVolumes or this code will be a mess.  Volumes
		// already does this right.  It will also help clean this code path up with less
		// conditionals. We do this when we integrate with libpod again.
		utils.Error(w, "not implemented", http.StatusInternalServerError, errors.New("not implemented"))
		return
	}
	for _, p := range pods {
		if err := runtime.RemovePod(r.Context(), p, true, query.force); err != nil {
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PodPause(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	_, err = pod.Pause()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PodUnpause(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 404 no such
	// 500 internal
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	_, err = pod.Unpause()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PodKill(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
		signal  = "SIGKILL"
	)
	query := struct {
		signal string `schema:"signal"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	muxVars := mux.Vars(r)
	if _, found := muxVars["signal"]; found {
		signal = query.signal
	}

	sig, err := util.ParseSignal(signal)
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "unable to parse signal value"))
	}
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	podStates, err := pod.Status()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	hasRunning := false
	for _, s := range podStates {
		if s == define.ContainerStateRunning {
			hasRunning = true
			break
		}
	}
	if !hasRunning {
		msg := fmt.Sprintf("Container %s is not running", pod.ID())
		utils.Error(w, msg, http.StatusConflict, errors.Errorf("cannot kill a pod with no running containers: %s", pod.ID()))
		return
	}
	_, err = pod.Kill(uint(sig))
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}

func PodExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	_, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, "")
}
