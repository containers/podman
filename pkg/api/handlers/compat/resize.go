package compat

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/common/pkg/resize"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

func ResizeTTY(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	// /containers/{id}/resize
	query := struct {
		Height           uint16 `schema:"h"`
		Width            uint16 `schema:"w"`
		IgnoreNotRunning bool   `schema:"running"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	sz := resize.TerminalSize{
		Width:  query.Width,
		Height: query.Height,
	}

	var status int
	switch {
	case strings.Contains(r.URL.Path, "/containers/"):
		name := utils.GetName(r)
		ctnr, err := runtime.LookupContainer(name)
		if err != nil {
			utils.ContainerNotFound(w, name, err)
			return
		}
		if err := ctnr.AttachResize(sz); err != nil {
			if !errors.Is(err, define.ErrCtrStateInvalid) {
				utils.InternalServerError(w, fmt.Errorf("cannot resize container: %w", err))
			} else {
				utils.Error(w, http.StatusConflict, err)
			}
			return
		}
		// This is not a 204, even though we write nothing, for compatibility
		// reasons.
		status = http.StatusOK
	case strings.Contains(r.URL.Path, "/exec/"):
		name := mux.Vars(r)["id"]
		ctnr, err := runtime.GetExecSessionContainer(name)
		if err != nil {
			utils.SessionNotFound(w, name, err)
			return
		}
		if state, err := ctnr.State(); err != nil {
			utils.InternalServerError(w, fmt.Errorf("cannot obtain session container state: %w", err))
			return
		} else if state != define.ContainerStateRunning && !query.IgnoreNotRunning {
			utils.Error(w, http.StatusConflict, fmt.Errorf("container %q in wrong state %q", name, state.String()))
			return
		}
		if err := ctnr.ExecResize(name, sz); err != nil {
			if !errors.Is(err, define.ErrExecSessionStateInvalid) || !query.IgnoreNotRunning {
				utils.InternalServerError(w, fmt.Errorf("cannot resize session: %w", err))
				return
			}
		}
		// This is not a 204, even though we write nothing, for compatibility
		// reasons.
		status = http.StatusCreated
	}
	w.WriteHeader(status)
}
