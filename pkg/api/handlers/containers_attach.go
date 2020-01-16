package handlers

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

func AttachContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		DetachKeys string `schema:"detachKeys"`
		Logs       bool   `schema:"logs"`
		Stream     bool   `schema:"stream"`
		Stdin      bool   `schema:"stdin"`
		Stdout     bool   `schema:"stdout"`
		Stderr     bool   `schema:"stderr"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Error parsing parameters", http.StatusBadRequest, err)
		return
	}

	muxVars := mux.Vars(r)

	// Detach keys: explicitly set to "" is very different from unset
	// TODO: Our format for parsing these may be different from Docker.
	var detachKeys *string
	if _, found := muxVars["detachKeys"]; found {
		detachKeys = &query.DetachKeys
	}

	streams := new(libpod.HTTPAttachStreams)
	streams.Stdout = true
	streams.Stderr = true
	streams.Stdin = true
	useStreams := false
	if _, found := muxVars["stdin"]; found {
		streams.Stdin = query.Stdin
		useStreams = true
	}
	if _, found := muxVars["stdout"]; found {
		streams.Stdout = query.Stdout
		useStreams = true
	}
	if _, found := muxVars["stderr"]; found {
		streams.Stderr = query.Stderr
		useStreams = true
	}
	if !useStreams {
		streams = nil
	}
	if useStreams && !streams.Stdout && !streams.Stderr && !streams.Stdin {
		utils.Error(w, "Parameter conflict", http.StatusBadRequest, errors.Errorf("at least one of stdin, stdout, stderr must be true"))
		return
	}

	// TODO: Investigate supporting these.
	// Logs replays container logs over the attach socket.
	// Stream seems to break things up somehow? Not 100% clear.
	if query.Logs {
		utils.Error(w, "Unsupported parameter", http.StatusBadRequest, errors.Errorf("the logs parameter to attach is not presently supported"))
		return
	}
	// We only support stream=true or unset
	if _, found := muxVars["stream"]; found && query.Stream {
		utils.Error(w, "Unsupported parameter", http.StatusBadRequest, errors.Errorf("the stream parameter to attach is not presently supported"))
		return
	}

	name := getName(r)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := ctr.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if !(state == define.ContainerStateCreated || state == define.ContainerStateRunning) {
		utils.InternalServerError(w, errors.Wrapf(define.ErrCtrStateInvalid, "can only attach to created or running containers"))
		return
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		utils.InternalServerError(w, errors.Errorf("unable to hijack connection"))
		return
	}

	w.WriteHeader(http.StatusSwitchingProtocols)

	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "error hijacking connection"))
		return
	}

	logrus.Debugf("Hijack for attach of container %s successful", ctr.ID())

	// Perform HTTP attach.
	// HTTPAttach will handle everything about the connection from here on
	// (including closing it and writing errors to it).
	if err := ctr.HTTPAttach(connection, buffer, streams, detachKeys, nil); err != nil {
		// We can't really do anything about errors anymore. HTTPAttach
		// should be writing them to the connection.
		logrus.Errorf("Error attaching to container %s: %v", ctr.ID(), err)
	}

	logrus.Debugf("Attach for container %s completed successfully", ctr.ID())
}

func ResizeContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Height uint16 `schema:"h"`
		Width  uint16 `schema:"w"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		// This is not a 400, despite the fact that is should be, for
		// compatibility reasons.
		utils.InternalServerError(w, errors.Wrapf(err, "error parsing query options"))
		return
	}

	name := getName(r)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	newSize := remotecommand.TerminalSize{
		Width:  query.Width,
		Height: query.Height,
	}
	if err := ctr.AttachResize(newSize); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// This is not a 204, even though we write nothing, for compatibility
	// reasons.
	utils.WriteResponse(w, http.StatusOK, "")
}
