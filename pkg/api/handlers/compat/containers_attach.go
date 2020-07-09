package compat

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/containers/libpod/v2/libpod"
	"github.com/containers/libpod/v2/libpod/define"
	"github.com/containers/libpod/v2/pkg/api/handlers/utils"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Error parsing parameters", http.StatusBadRequest, err)
		return
	}

	// Detach keys: explicitly set to "" is very different from unset
	// TODO: Our format for parsing these may be different from Docker.
	var detachKeys *string
	if _, found := r.URL.Query()["detachKeys"]; found {
		detachKeys = &query.DetachKeys
	}

	streams := new(libpod.HTTPAttachStreams)
	streams.Stdout = true
	streams.Stderr = true
	streams.Stdin = true
	useStreams := false
	if _, found := r.URL.Query()["stdin"]; found {
		streams.Stdin = query.Stdin
		useStreams = true
	}
	if _, found := r.URL.Query()["stdout"]; found {
		streams.Stdout = query.Stdout
		useStreams = true
	}
	if _, found := r.URL.Query()["stderr"]; found {
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

	// At least one of these must be set
	if !query.Stream && !query.Logs {
		utils.Error(w, "Unsupported parameter", http.StatusBadRequest, errors.Errorf("at least one of Logs or Stream must be set"))
		return
	}

	name := utils.GetName(r)
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
	// For Docker compatibility, we need to re-initialize containers in these states.
	if state == define.ContainerStateConfigured || state == define.ContainerStateExited {
		if err := ctr.Init(r.Context(), ctr.PodID() != ""); err != nil {
			utils.Error(w, "Container in wrong state", http.StatusConflict, errors.Wrapf(err, "error preparing container %s for attach", ctr.ID()))
			return
		}
	} else if !(state == define.ContainerStateCreated || state == define.ContainerStateRunning) {
		utils.InternalServerError(w, errors.Wrapf(define.ErrCtrStateInvalid, "can only attach to created or running containers - currently in state %s", state.String()))
		return
	}

	connection, buffer, err := AttachConnection(w, r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	logrus.Debugf("Hijack for attach of container %s successful", ctr.ID())

	// Perform HTTP attach.
	// HTTPAttach will handle everything about the connection from here on
	// (including closing it and writing errors to it).
	if err := ctr.HTTPAttach(connection, buffer, streams, detachKeys, nil, query.Stream, query.Logs); err != nil {
		// We can't really do anything about errors anymore. HTTPAttach
		// should be writing them to the connection.
		logrus.Errorf("Error attaching to container %s: %v", ctr.ID(), err)
	}

	logrus.Debugf("Attach for container %s completed successfully", ctr.ID())
}

func AttachConnection(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.ReadWriter, error) {
	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.Errorf("unable to hijack connection")
	}

	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error hijacking connection")
	}

	WriteAttachHeaders(r, connection)

	return connection, buffer, nil
}

func WriteAttachHeaders(r *http.Request, connection io.Writer) {
	// AttachHeader is the literal header sent for upgraded/hijacked connections for
	// attach, sourced from Docker at:
	// https://raw.githubusercontent.com/moby/moby/b95fad8e51bd064be4f4e58a996924f343846c85/api/server/router/container/container_routes.go
	// Using literally to ensure compatibility with existing clients.
	c := r.Header.Get("Connection")
	proto := r.Header.Get("Upgrade")
	if len(proto) == 0 || !strings.EqualFold(c, "Upgrade") {
		// OK - can't upgrade if not requested or protocol is not specified
		fmt.Fprintf(connection,
			"HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	} else {
		// Upraded
		fmt.Fprintf(connection,
			"HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n",
			proto)
	}
}
