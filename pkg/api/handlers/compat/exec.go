package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecCreateHandler creates an exec session for a given container.
func ExecCreateHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	input := new(handlers.ExecCreateConfig)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "error decoding request body as JSON"))
		return
	}

	ctrName := utils.GetName(r)
	ctr, err := runtime.LookupContainer(ctrName)
	if err != nil {
		utils.ContainerNotFound(w, ctrName, err)
		return
	}

	libpodConfig := new(libpod.ExecConfig)
	libpodConfig.Command = input.Cmd
	libpodConfig.Terminal = input.Tty
	libpodConfig.AttachStdin = input.AttachStdin
	libpodConfig.AttachStderr = input.AttachStderr
	libpodConfig.AttachStdout = input.AttachStdout
	if input.DetachKeys != "" {
		libpodConfig.DetachKeys = &input.DetachKeys
	}
	libpodConfig.Environment = make(map[string]string)
	for _, envStr := range input.Env {
		split := strings.SplitN(envStr, "=", 2)
		if len(split) != 2 {
			utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, errors.Errorf("environment variable %q badly formed, must be key=value", envStr))
			return
		}
		libpodConfig.Environment[split[0]] = split[1]
	}
	libpodConfig.WorkDir = input.WorkingDir
	libpodConfig.Privileged = input.Privileged
	libpodConfig.User = input.User

	sessID, err := ctr.ExecCreate(libpodConfig)
	if err != nil {
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			// Check if the container is paused. If so, return a 409
			state, err := ctr.State()
			if err == nil {
				// Ignore the error != nil case. We're already
				// throwing an InternalServerError below.
				if state == define.ContainerStatePaused {
					utils.Error(w, "Container is paused", http.StatusConflict, errors.Errorf("cannot create exec session as container %s is paused", ctr.ID()))
					return
				}
			}
		}
		utils.InternalServerError(w, err)
		return
	}

	resp := new(handlers.ExecCreateResponse)
	resp.ID = sessID

	utils.WriteResponse(w, http.StatusCreated, resp)
}

// ExecInspectHandler inspects a given exec session.
func ExecInspectHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]
	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, fmt.Sprintf("No such exec session: %s", sessionID), http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Inspecting exec session %s of container %s", sessionID, sessionCtr.ID())

	session, err := sessionCtr.ExecSession(sessionID)
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "error retrieving exec session %s from container %s", sessionID, sessionCtr.ID()))
		return
	}

	inspectOut, err := session.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, inspectOut)

	// Only for the Compat API: we want to remove sessions that were
	// stopped. This is very hacky, but should suffice for now.
	if !utils.IsLibpodRequest(r) && inspectOut.CanRemove {
		logrus.Infof("Pruning stale exec session %s from container %s", sessionID, sessionCtr.ID())
		if err := sessionCtr.ExecRemove(sessionID, false); err != nil && errors.Cause(err) != define.ErrNoSuchExecSession {
			logrus.Errorf("Error removing stale exec session %s from container %s: %v", sessionID, sessionCtr.ID(), err)
		}
	}
}

// ExecResizeHandler resizes a given exec session's TTY.
func ExecResizeHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	sessionID := mux.Vars(r)["id"]

	query := struct {
		Height uint16 `schema:"h"`
		Width  uint16 `schema:"w"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, fmt.Sprintf("No such exec session: %s", sessionID), http.StatusNotFound, err)
		return
	}

	newSize := remotecommand.TerminalSize{
		Width:  query.Width,
		Height: query.Height,
	}

	if err := sessionCtr.ExecResize(sessionID, newSize); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// This is a 201 some reason, not a 204.
	utils.WriteResponse(w, http.StatusCreated, "")
}

// ExecStartHandler runs a given exec session.
func ExecStartHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]

	// TODO: We should read/support Tty and Detach from here.
	bodyParams := new(handlers.ExecStartConfig)

	if err := json.NewDecoder(r.Body).Decode(&bodyParams); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to decode parameters for %s", r.URL.String()))
		return
	}
	if bodyParams.Detach == true {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Errorf("Detached exec is not yet supported"))
		return
	}
	// TODO: Verify TTY setting against what inspect session was made with

	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, fmt.Sprintf("No such exec session: %s", sessionID), http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Starting exec session %s of container %s", sessionID, sessionCtr.ID())

	state, err := sessionCtr.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning {
		utils.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict, errors.Errorf("cannot exec in a container that is not running; container %s is %s", sessionCtr.ID(), state.String()))
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		utils.InternalServerError(w, errors.Errorf("unable to hijack connection"))
		return
	}

	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "error hijacking connection"))
		return
	}

	fmt.Fprintf(connection, AttachHeader)

	logrus.Debugf("Hijack for attach of container %s exec session %s successful", sessionCtr.ID(), sessionID)

	if err := sessionCtr.ExecHTTPStartAndAttach(sessionID, connection, buffer, nil, nil, nil); err != nil {
		logrus.Errorf("Error attaching to container %s exec session %s: %v", sessionCtr.ID(), sessionID, err)
	}

	logrus.Debugf("Attach for container %s exec session %s completed successfully", sessionCtr.ID(), sessionID)
}
