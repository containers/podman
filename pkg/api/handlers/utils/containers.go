package utils

import (
	"context"
	"net/http"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// ContainerCreateResponse is the response struct for creating a container
type ContainerCreateResponse struct {
	// ID of the container created
	ID string `json:"Id"`
	// Warnings during container creation
	Warnings []string `json:"Warnings"`
}

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

// GenerateFilterFuncsFromMap is used to generate un-executed functions that can be used to filter
// containers.  It is specifically designed for the RESTFUL API input.
func GenerateFilterFuncsFromMap(r *libpod.Runtime, filters map[string][]string) ([]libpod.ContainerFilter, error) {
	var (
		filterFuncs []libpod.ContainerFilter
	)
	for k, v := range filters {
		for _, val := range v {
			f, err := shared.GenerateContainerFilterFuncs(k, val, r)
			if err != nil {
				return filterFuncs, err
			}
			filterFuncs = append(filterFuncs, f)
		}
	}
	return filterFuncs, nil
}

func CreateContainer(ctx context.Context, w http.ResponseWriter, runtime *libpod.Runtime, cc *createconfig.CreateConfig) {
	var pod *libpod.Pod
	ctr, err := shared.CreateContainerFromCreateConfig(runtime, cc, ctx, pod)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "CreateContainerFromCreateConfig()"))
		return
	}

	response := ContainerCreateResponse{
		ID:       ctr.ID(),
		Warnings: []string{}}

	WriteResponse(w, http.StatusCreated, response)
}
