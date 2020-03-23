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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
}
