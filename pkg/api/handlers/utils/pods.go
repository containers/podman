package utils

import (
	"net/http"

	"github.com/containers/podman/v2/libpod"
	lpfilters "github.com/containers/podman/v2/libpod/filters"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/gorilla/schema"
)

func GetPods(w http.ResponseWriter, r *http.Request) ([]*entities.ListPodsReport, error) {
	var (
		pods    []*libpod.Pod
		filters []libpod.PodFilter
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
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		UnSupportedParameter("digests")
	}

	for k, v := range query.Filters {
		for _, filter := range v {
			f, err := lpfilters.GeneratePodFilterFunc(k, filter)
			if err != nil {
				return nil, err
			}
			filters = append(filters, f)
		}
	}
	pods, err := runtime.Pods(filters...)
	if err != nil {
		return nil, err
	}

	if len(pods) == 0 {
		return []*entities.ListPodsReport{}, nil
	}

	lps := make([]*entities.ListPodsReport, 0, len(pods))
	for _, pod := range pods {
		status, err := pod.GetPodStatus()
		if err != nil {
			return nil, err
		}
		ctrs, err := pod.AllContainers()
		if err != nil {
			return nil, err
		}
		infraID, err := pod.InfraContainerID()
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
			InfraId:   infraID,
			Labels:    pod.Labels(),
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
