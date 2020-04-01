package utils

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/gorilla/schema"
)

func GetPods(w http.ResponseWriter, r *http.Request) ([]*entities.ListPodsReport, error) {
	var (
		lps    []*entities.ListPodsReport
		pods   []*libpod.Pod
		podErr error
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		All     bool
		Filters map[string][]string `schema:"filters"`
		Digests bool
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		return nil, err
	}
	var filters = []string{}
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		UnSupportedParameter("digests")
	}

	if len(query.Filters) > 0 {
		for k, v := range query.Filters {
			for _, val := range v {
				filters = append(filters, fmt.Sprintf("%s=%s", k, val))
			}
		}
		filterFuncs, err := shared.GenerateFilterFunction(runtime, filters)
		if err != nil {
			return nil, err
		}
		pods, podErr = shared.FilterAllPodsWithFilterFunc(runtime, filterFuncs...)
	} else {
		pods, podErr = runtime.GetAllPods()
	}
	if podErr != nil {
		return nil, podErr
	}
	for _, pod := range pods {
		status, err := pod.GetPodStatus()
		if err != nil {
			return nil, err
		}
		ctrs, err := pod.AllContainers()
		if err != nil {
			return nil, err
		}
		infraId, err := pod.InfraContainerID()
		if err != nil {
			return nil, err
		}
		lp := entities.ListPodsReport{
			Cgroup:    pod.CgroupParent(),
			Created:   pod.CreatedTime(),
			Id:        pod.ID(),
			Name:      pod.Name(),
			Namespace: pod.Namespace(),
			Status:    status,
			InfraId:   infraId,
		}
		for _, ctr := range ctrs {
			state, err := ctr.State()
			if err != nil {
				return nil, err
			}
			lp.Containers = append(lp.Containers, &entities.ListPodContainer{
				Id:     ctr.ID(),
				Names:  ctr.Name(),
				Status: state.String(),
			})
		}
		lps = append(lps, &lp)
	}
	return lps, nil
}
