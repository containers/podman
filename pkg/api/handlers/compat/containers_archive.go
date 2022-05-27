package compat

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/copy"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	switch r.Method {
	case http.MethodPut:
		handlePut(w, r, decoder, runtime)
	case http.MethodHead, http.MethodGet:
		handleHeadAndGet(w, r, decoder, runtime)
	default:
		utils.Error(w, http.StatusNotImplemented, errors.Errorf("unsupported method: %v", r.Method))
	}
}

func handleHeadAndGet(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path string `schema:"path"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	if query.Path == "" {
		utils.Error(w, http.StatusBadRequest, errors.New("missing `path` parameter"))
		return
	}

	containerName := utils.GetName(r)
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	statReport, err := containerEngine.ContainerStat(r.Context(), containerName, query.Path)

	// NOTE
	// The statReport may actually be set even in case of an error.  That's
	// the case when we're looking at a symlink pointing to nirvana.  In
	// such cases, we really need the FileInfo but we also need the error.
	if statReport != nil {
		statHeader, err := copy.EncodeFileInfo(&statReport.FileInfo)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Add(copy.XDockerContainerPathStatHeader, statHeader)
	}

	if errors.Cause(err) == define.ErrNoSuchCtr || errors.Cause(err) == copy.ErrENOENT {
		// 404 is returned for an absent container and path.  The
		// clients must deal with it accordingly.
		utils.Error(w, http.StatusNotFound, err)
		return
	} else if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Our work is done when the user is interested in the header only.
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	copyFunc, err := containerEngine.ContainerCopyToArchive(r.Context(), containerName, query.Path, w)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.WriteHeader(http.StatusOK)
	if err := copyFunc(); err != nil {
		logrus.Error(err.Error())
	}
}

func handlePut(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path                 string `schema:"path"`
		Chown                bool   `schema:"copyUIDGID"`
		Rename               string `schema:"rename"`
		NoOverwriteDirNonDir bool   `schema:"noOverwriteDirNonDir"`
	}{
		Chown: utils.IsLibpodRequest(r), // backward compatibility
	}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	var rename map[string]string
	if query.Rename != "" {
		if err := json.Unmarshal([]byte(query.Rename), &rename); err != nil {
			utils.Error(w, http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query field 'rename'"))
			return
		}
	}

	containerName := utils.GetName(r)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	copyFunc, err := containerEngine.ContainerCopyFromArchive(r.Context(), containerName, query.Path, r.Body,
		entities.CopyOptions{
			Chown:                query.Chown,
			NoOverwriteDirNonDir: query.NoOverwriteDirNonDir,
			Rename:               rename,
		})
	if err != nil {
		switch {
		case errors.Cause(err) == define.ErrNoSuchCtr || os.IsNotExist(err):
			// 404 is returned for an absent container and path.  The
			// clients must deal with it accordingly.
			utils.Error(w, http.StatusNotFound, errors.Wrap(err, "the container doesn't exists"))
		case strings.Contains(err.Error(), "copier: put: error creating file"):
			// Not the best test but need to break this out for compatibility
			// See vendor/github.com/containers/buildah/copier/copier.go:1585
			utils.Error(w, http.StatusBadRequest, err)
		default:
			utils.Error(w, http.StatusInternalServerError, err)
		}
		return
	}

	if err := copyFunc(); err != nil {
		logrus.Error(err.Error())
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
