package serviceapi

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"strconv"
	"syscall"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/docker/docker/pkg/signal"
	"github.com/gorilla/mux"
)

func registerPodsHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/libpod/pods/"), serviceHandler(pods))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}"), serviceHandler(podDelete)).Methods("DELETE")
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/exists"), serviceHandler(podExists))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/kill"), serviceHandler(podKill))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/pause"), serviceHandler(podPause))
	r.Handle(unversionedPath("/libpod/pods/prune"), serviceHandler(podPrune))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/unpause"), serviceHandler(podUnpause))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/restart"), serviceHandler(podRestart))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/start"), serviceHandler(podStart))
	r.Handle(unversionedPath("/libpod/pods/{name:..*}/stop"), serviceHandler(podStop))
	return nil
}

func pods(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}

func podStop(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	var (
		stopError error
	)
	allContainersStopped := true
	ctx := context.Background()
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
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
		_, stopError = pod.StopWithTimeout(ctx, false, timeout)
	} else {
		_, stopError = pod.Stop(ctx, false)
	}
	if stopError != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podStart(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	allContainersRunning := true
	ctx := context.Background()
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
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
	if _, err := pod.Start(ctx); err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podDelete(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	ctx := context.Background()
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
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
	if err := runtime.RemovePod(ctx, pod, true, force); err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podRestart(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	ctx := context.Background()
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
		return
	}
	_, err = pod.Restart(ctx)
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podPrune(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	ctx := context.Background()
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
		pods, err = runtime.GetAllPods()
	} else {
		// TODO We need to make a libpod.PruneVolumes or this code will be a mess.  Volumes
		// already does this right.  It will also help clean this code path up with less
		// conditionals. We do this when we integrate with libpod again.
		Error(w, "not implemented", http.StatusInternalServerError, nil)
		return
	}
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	for _, p := range pods {
		if err := runtime.RemovePod(ctx, p, true, force); err != nil {
			Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podPause(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
		return
	}
	_, err = pod.Pause()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podUnpause(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
		return
	}
	_, err = pod.Unpause()
	if err != nil {
		Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podKill(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]
	pod, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
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
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}

func podExists(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]

	_, err := runtime.LookupPod(name)
	if err != nil {
		noSuchPodError(w, name, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, "")
}
