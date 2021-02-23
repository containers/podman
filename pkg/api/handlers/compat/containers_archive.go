package compat

import (
	"fmt"
	"net/http"
	"os"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/copy"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	switch r.Method {
	case http.MethodPut:
		handlePut(w, r, decoder, runtime)
	case http.MethodHead, http.MethodGet:
		handleHeadAndGet(w, r, decoder, runtime)
	default:
		utils.Error(w, fmt.Sprintf("unsupported method: %v", r.Method), http.StatusNotImplemented, errors.New(fmt.Sprintf("unsupported method: %v", r.Method)))
	}
}

func handleHeadAndGet(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path string `schema:"path"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	if query.Path == "" {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.New("missing `path` parameter"))
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
			utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
			return
		}
		w.Header().Add(copy.XDockerContainerPathStatHeader, statHeader)
	}

	if errors.Cause(err) == define.ErrNoSuchCtr || errors.Cause(err) == copy.ErrENOENT {
		// 404 is returned for an absent container and path.  The
		// clients must deal with it accordingly.
		utils.Error(w, "Not found.", http.StatusNotFound, err)
		return
	} else if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Our work is done when the user is interested in the header only.
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	copyFunc, err := containerEngine.ContainerCopyToArchive(r.Context(), containerName, query.Path, w)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
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
		Path string `schema:"path"`
		// TODO handle params below
		NoOverwriteDirNonDir bool `schema:"noOverwriteDirNonDir"`
		CopyUIDGID           bool `schema:"copyUIDGID"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	containerName := utils.GetName(r)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	copyFunc, err := containerEngine.ContainerCopyFromArchive(r.Context(), containerName, query.Path, r.Body)
	if errors.Cause(err) == define.ErrNoSuchCtr || os.IsNotExist(err) {
		// 404 is returned for an absent container and path.  The
		// clients must deal with it accordingly.
		utils.Error(w, "Not found.", http.StatusNotFound, errors.Wrap(err, "the container doesn't exists"))
		return
	} else if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := copyFunc(); err != nil {
		logrus.Error(err.Error())
	}
}
