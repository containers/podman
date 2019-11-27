package serviceapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/docker/docker/pkg/signal"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/libpod/pods/"), s.serviceHandler(s.pods))
	r.Handle(versionedPath("/libpod/pods/create"), s.serviceHandler(s.podCreate))
	r.Handle(versionedPath("/libpod/pods/prune"), s.serviceHandler(s.podPrune))
	r.Handle(versionedPath("/libpod/pods/{name:..*}"), s.serviceHandler(s.podDelete)).Methods("DELETE")
	r.Handle(versionedPath("/libpod/pods/{name:..*}"), s.serviceHandler(s.podInspect)).Methods("GET")
	r.Handle(versionedPath("/libpod/pods/{name:..*}/exists"), s.serviceHandler(s.podExists))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/kill"), s.serviceHandler(s.podKill))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/pause"), s.serviceHandler(s.podPause))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/unpause"), s.serviceHandler(s.podUnpause))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/restart"), s.serviceHandler(s.podRestart))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/start"), s.serviceHandler(s.podStart))
	r.Handle(versionedPath("/libpod/pods/{name:..*}/stop"), s.serviceHandler(s.podStop))
	return nil
}

func (s *APIServer) podCreate(w http.ResponseWriter, r *http.Request) {
	var (
		options []libpod.PodCreateOption
		err     error
	)
	labels := make(map[string]string)
	input := PodCreate{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if len(input.InfraCommand) > 0 || len(input.InfraImage) > 0 {
		Error(w, "Something went wrong.", http.StatusInternalServerError,
			errors.New("infra-command and infra-image are not implemented yet"))
		return
	}
	//TODO long term we should break the following out of adapter and into libpod proper
	// so that the cli and api can share the creation of a pod with the same options
	if len(input.CGroupParent) > 0 {
		options = append(options, libpod.WithPodCgroupParent(input.CGroupParent))
	}

	if len(input.Labels) > 0 {
		if err := parse.ReadKVStrings(labels, []string{}, input.Labels); err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
		//TODO infra-image and infra-command are not supported in the libpod API yet.  Will fix
		// when implemented in libpod
		options = append(options, libpod.WithInfraContainer())
		sharedNamespaces := shared.DefaultKernelNamespaces
		if len(input.Share) > 0 {
			sharedNamespaces = input.Share
		}
		nsOptions, err := shared.GetNamespaceOptions(strings.Split(sharedNamespaces, ","))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
		options = append(options, nsOptions...)
	}

	if len(input.Publish) > 0 {
		portBindings, err := shared.CreatePortBindings(input.Publish)
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	// always have containers use pod cgroups
	// User Opt out is not yet supported
	options = append(options, libpod.WithPodCgroups())

	pod, err := s.Runtime.NewPod(s.Context, options...)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusCreated, IDResponse{ID: pod.CgroupParent()})
}

func (s *APIServer) pods(w http.ResponseWriter, r *http.Request) {
	var podInspectData []*libpod.PodInspect

	filters := r.Form.Get("filter")
	if len(filters) > 0 {
		Error(w, "filters are not implemented yet", http.StatusInternalServerError, define.ErrNotImplemented)
		return
	}

	pods, err := s.Runtime.GetAllPods()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	for _, pod := range pods {
		data, err := pod.Inspect()
		if err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		podInspectData = append(podInspectData, data)
	}
	s.WriteResponse(w, http.StatusOK, podInspectData)
}

func (s *APIServer) podInspect(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}

	podData, err := pod.Inspect()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, podData)
}

func (s *APIServer) podStop(w http.ResponseWriter, r *http.Request) {
	var (
		stopError error
	)
	allContainersStopped := true
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}

	// TODO we need to implement a pod.State/Status in libpod internal so libpod api
	// users dont have to run through all containers.
	podContainers, err := pod.AllContainers()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	for _, con := range podContainers {
		containerState, err := con.State()
		if err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		if containerState == define.ContainerStateRunning {
			allContainersStopped = false
			break
		}
	}
	if allContainersStopped {
		alreadyStopped := errors.Errorf("pod %s is already stopped", pod.ID())
		Error(w, "Something went wrong", http.StatusNotModified, alreadyStopped)
		return
	}

	if len(r.Form.Get("t")) > 0 {
		timeout, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		_, stopError = pod.StopWithTimeout(s.Context, false, timeout)
	} else {
		_, stopError = pod.Stop(s.Context, false)
	}
	if stopError != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podStart(w http.ResponseWriter, r *http.Request) {
	allContainersRunning := true
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}

	// TODO we need to implement a pod.State/Status in libpod internal so libpod api
	// users dont have to run through all containers.
	podContainers, err := pod.AllContainers()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	for _, con := range podContainers {
		containerState, err := con.State()
		if err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		if containerState != define.ContainerStateRunning {
			allContainersRunning = false
			break
		}
	}
	if allContainersRunning {
		alreadyRunning := errors.Errorf("pod %s is already running", pod.ID())
		Error(w, "Something went wrong", http.StatusNotModified, alreadyRunning)
		return
	}
	if _, err := pod.Start(s.Context); err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podDelete(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	force := false
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			// If the parameter is bad, we pass back a 400
			Error(w, "Something went wrong", http.StatusBadRequest, err)
			return
		}
	}
	if err := s.Runtime.RemovePod(s.Context, pod, true, force); err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podRestart(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	_, err = pod.Restart(s.Context)
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podPrune(w http.ResponseWriter, r *http.Request) {
	var (
		pods  []*libpod.Pod
		force bool
		err   error
	)
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
	}
	if force {
		pods, err = s.Runtime.GetAllPods()
	} else {
		// TODO We need to make a libpod.PruneVolumes or this code will be a mess.  Volumes
		// already does this right.  It will also help clean this code path up with less
		// conditionals. We do this when we integrate with libpod again.
		Error(w, "not implemented", http.StatusInternalServerError, errors.New("not implemented"))
		return
	}
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	for _, p := range pods {
		if err := s.Runtime.RemovePod(s.Context, p, true, force); err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podPause(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	_, err = pod.Pause()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podUnpause(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	_, err = pod.Unpause()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podKill(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	pod, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	podStates, err := pod.Status()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
		Error(w, msg, http.StatusConflict, errors.Errorf("cannot kill a pod with no running containers: %s", pod.ID()))
		return
	}
	sig := syscall.SIGKILL
	if len(r.Form.Get("signal")) > 0 {
		sig, err = signal.ParseSignal(r.Form.Get("signal"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "unable to parse signal %s", r.Form.Get("signal")))
			return
		}
	}
	_, err = pod.Kill(uint(sig))
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}

func (s *APIServer) podExists(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	_, err := s.Runtime.LookupPod(name)
	if err != nil {
		podNotFound(w, name, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, "")
}
