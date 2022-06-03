package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

var (
	stopOptions = entities.StopOptions{
		Filters: make(map[string][]string),
	}
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handlers/compat")
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	//testQuery := url.Values{}
	test := []*entities.StopOptions{}
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	fmt.Println("line 24")
	// /{version}/containers/(name)/stop
	query := struct {
		Ignore        bool `schema:"ignore"`
		DockerTimeout uint `schema:"t"`
		LibpodTimeout uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	fmt.Println("line 37")
	//fmt.Println(entities.StopOptions.Filters)
	name := utils.GetName(r)
	fmt.Println(test)
	options := entities.StopOptions{
		Filters: make(map[string][]string),
		Ignore:  query.Ignore,
	}
	fmt.Println(options.Filters)

	if utils.IsLibpodRequest(r) {
		if _, found := r.URL.Query()["timeout"]; found {
			options.Timeout = &query.LibpodTimeout
		}
	} else {
		if _, found := r.URL.Query()["t"]; found {
			options.Timeout = &query.DockerTimeout
		}
	}
	con, err := runtime.LookupContainer(name)
	if err != nil {
		fmt.Println("containerNotFound")
		utils.ContainerNotFound(w, name, err)
		return
	}
	state, err := con.State()
	if err != nil {
		fmt.Println("server error")
		utils.InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		fmt.Println("writeresponse")
		utils.WriteResponse(w, http.StatusNotModified, nil)
		return
	}
	fmt.Println("at container stop call")
	fmt.Println(name)
	fmt.Println(len(options.Filters))
	fmt.Println(options.All)
	fmt.Println(options.Filters)
	fmt.Println(stopOptions.All)
	report, err := containerEngine.ContainerStop(r.Context(), []string{name}, options)
	//err = con.Stop()
	fmt.Println(name)
	if err != nil {
		fmt.Println("there is an error")
		if errors.Cause(err) == define.ErrNoSuchCtr {
			fmt.Println("container not found")
			utils.ContainerNotFound(w, name, err)
			return
		}
		fmt.Println("internal server error")
		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		utils.InternalServerError(w, report[0].Err)
		return
	}
	fmt.Println("success")
	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
