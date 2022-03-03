package compat

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	"github.com/containers/podman/v4/pkg/api/server/idle"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExecCreateHandler creates an exec session for a given container.
func ExecCreateHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

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
			utils.Error(w, http.StatusBadRequest, errors.Errorf("environment variable %q badly formed, must be key=value", envStr))
			return
		}
		libpodConfig.Environment[split[0]] = split[1]
	}
	libpodConfig.WorkDir = input.WorkingDir
	libpodConfig.Privileged = input.Privileged
	libpodConfig.User = input.User

	// Make our exit command
	storageConfig := runtime.StorageConfig()
	runtimeConfig, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// Automatically log to syslog if the server has log-level=debug set
	exitCommandArgs, err := specgenutil.CreateExitCommandArgs(storageConfig, runtimeConfig, logrus.IsLevelEnabled(logrus.DebugLevel), true, true)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	libpodConfig.ExitCommand = exitCommandArgs

	// Run the exit command after 5 minutes, to mimic Docker's exec cleanup
	// behavior.
	libpodConfig.ExitCommandDelay = runtimeConfig.Engine.ExitCommandDelay

	sessID, err := ctr.ExecCreate(libpodConfig)
	if err != nil {
		if errors.Cause(err) == define.ErrCtrStateInvalid {
			// Check if the container is paused. If so, return a 409
			state, err := ctr.State()
			if err == nil {
				// Ignore the error != nil case. We're already
				// throwing an InternalServerError below.
				if state == define.ContainerStatePaused {
					utils.Error(w, http.StatusConflict, errors.Errorf("cannot create exec session as container %s is paused", ctr.ID()))
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
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]
	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
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

// ExecStartHandler runs a given exec session.
func ExecStartHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]

	// TODO: We should read/support Tty from here.
	bodyParams := new(handlers.ExecStartConfig)

	if err := json.NewDecoder(r.Body).Decode(&bodyParams); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to decode parameters for %s", r.URL.String()))
		return
	}
	// TODO: Verify TTY setting against what inspect session was made with

	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Starting exec session %s of container %s", sessionID, sessionCtr.ID())

	state, err := sessionCtr.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning {
		utils.Error(w, http.StatusConflict, errors.Errorf("cannot exec in a container that is not running; container %s is %s", sessionCtr.ID(), state.String()))
		return
	}

	if bodyParams.Detach {
		// If we are detaching, we do NOT want to hijack.
		// Instead, we perform a detached start, and return 200 if
		// successful.
		if err := sessionCtr.ExecStart(sessionID); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		// This is a 200 despite having no content
		utils.WriteResponse(w, http.StatusOK, "")
		return
	}

	logErr := func(e error) {
		logrus.Error(errors.Wrapf(e, "error attaching to container %s exec session %s", sessionCtr.ID(), sessionID))
	}

	var size *define.TerminalSize
	if bodyParams.Tty && (bodyParams.Height > 0 || bodyParams.Width > 0) {
		size = &define.TerminalSize{
			Height: bodyParams.Height,
			Width:  bodyParams.Width,
		}
	}

	hijackChan := make(chan bool, 1)
	err = sessionCtr.ExecHTTPStartAndAttach(sessionID, r, w, nil, nil, nil, hijackChan, size)

	if <-hijackChan {
		// If connection was Hijacked, we have to signal it's being closed
		t := r.Context().Value(api.IdleTrackerKey).(*idle.Tracker)
		defer t.Close()

		if err != nil {
			// Cannot report error to client as a 500 as the Upgrade set status to 101
			logErr(err)
		}
	} else {
		// If the Hijack failed we are going to assume we can still inform client of failure
		utils.InternalServerError(w, err)
		logErr(err)
	}
	logrus.Debugf("Attach for container %s exec session %s completed successfully", sessionCtr.ID(), sessionID)
}
