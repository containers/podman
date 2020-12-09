package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/copy"
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

	ctr, err := runtime.LookupContainer(containerName)
	if errors.Cause(err) == define.ErrNoSuchCtr {
		utils.Error(w, "Not found.", http.StatusNotFound, errors.Wrap(err, "the container doesn't exists"))
		return
	} else if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	source, err := copy.CopyItemForContainer(ctr, query.Path, true, true)
	defer source.CleanUp()
	if err != nil {
		utils.Error(w, "Not found.", http.StatusNotFound, errors.Wrapf(err, "error stating container path %q", query.Path))
		return
	}

	// NOTE: Docker always sets the header.
	info, err := source.Stat()
	if err != nil {
		utils.Error(w, "Not found.", http.StatusNotFound, errors.Wrapf(err, "error stating container path %q", query.Path))
		return
	}
	statHeader, err := copy.EncodeFileInfo(info)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.Header().Add(copy.XDockerContainerPathStatHeader, statHeader)

	// Our work is done when the user is interested in the header only.
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Alright, the users wants data from the container.
	destination, err := copy.CopyItemForWriter(w)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	copier, err := copy.GetCopier(&source, &destination, false)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := copier.Copy(); err != nil {
		logrus.Errorf("Error during copy: %v", err)
		return
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

	ctrName := utils.GetName(r)

	ctr, err := runtime.LookupContainer(ctrName)
	if err != nil {
		utils.Error(w, "Not found", http.StatusNotFound, errors.Wrapf(err, "the %s container doesn't exists", ctrName))
		return
	}

	destination, err := copy.CopyItemForContainer(ctr, query.Path, true, false)
	defer destination.CleanUp()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	source, err := copy.CopyItemForReader(r.Body)
	defer source.CleanUp()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	copier, err := copy.GetCopier(&source, &destination, false)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := copier.Copy(); err != nil {
		logrus.Errorf("Error during copy: %v", err)
		return
	}
}
