package utils

import (
	"context"
	"net/http"
	"time"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	createconfig "github.com/containers/podman/v2/pkg/spec"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func WaitContainer(w http.ResponseWriter, r *http.Request) (int32, error) {
	var (
		err      error
		interval time.Duration
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Interval  string `schema:"interval"`
		Condition string `schema:"condition"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return 0, err
	}
	if _, found := r.URL.Query()["interval"]; found {
		interval, err = time.ParseDuration(query.Interval)
		if err != nil {
			InternalServerError(w, err)
			return 0, err
		}
	} else {
		interval, err = time.ParseDuration("250ms")
		if err != nil {
			InternalServerError(w, err)
			return 0, err
		}
	}
	condition := define.ContainerStateStopped
	if _, found := r.URL.Query()["condition"]; found {
		condition, err = define.StringToContainerStatus(query.Condition)
		if err != nil {
			InternalServerError(w, err)
			return 0, err
		}
	}
	name := GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return 0, err
	}
	return con.WaitForConditionWithInterval(interval, condition)
}

func CreateContainer(ctx context.Context, w http.ResponseWriter, runtime *libpod.Runtime, cc *createconfig.CreateConfig) {
	var pod *libpod.Pod
	ctr, err := createconfig.CreateContainerFromCreateConfig(ctx, runtime, cc, pod)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "CreateContainerFromCreateConfig()"))
		return
	}

	response := entities.ContainerCreateResponse{
		ID:       ctr.ID(),
		Warnings: []string{}}

	WriteResponse(w, http.StatusCreated, response)
}
