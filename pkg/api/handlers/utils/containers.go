package utils

import (
	"net/http"
	"time"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func WaitContainer(w http.ResponseWriter, r *http.Request) (int32, error) {
	var (
		err      error
		interval time.Duration
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Interval  string                   `schema:"interval"`
		Condition []define.ContainerStatus `schema:"condition"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return 0, err
	}
	options := entities.WaitOptions{
		Condition: []define.ContainerStatus{define.ContainerStateStopped},
	}
	name := GetName(r)
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
	options.Interval = interval

	if _, found := r.URL.Query()["condition"]; found {
		options.Condition = query.Condition
	}

	report, err := containerEngine.ContainerWait(r.Context(), []string{name}, options)
	if err != nil {
		return 0, err
	}
	if len(report) == 0 {
		InternalServerError(w, errors.New("No reports returned"))
		return 0, err
	}
	return report[0].ExitCode, report[0].Error
}
