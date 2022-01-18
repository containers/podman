package filters

import (
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/pkg/errors"
)

// GeneratePodFilterFunc takes a filter and filtervalue (key, value)
// and generates a libpod function that can be used to filter
// pods
func GeneratePodFilterFunc(filter string, filterValues []string, r *libpod.Runtime) (
	func(pod *libpod.Pod) bool, error) {
	switch filter {
	case "ctr-ids":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
			if err != nil {
				return false
			}
			for _, id := range ctrIds {
				return util.StringMatchRegexSlice(id, filterValues)
			}
			return false
		}, nil
	case "ctr-names":
		return func(p *libpod.Pod) bool {
			ctrs, err := p.AllContainers()
			if err != nil {
				return false
			}
			for _, ctr := range ctrs {
				return util.StringMatchRegexSlice(ctr.Name(), filterValues)
			}
			return false
		}, nil
	case "ctr-number":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
			if err != nil {
				return false
			}
			for _, filterValue := range filterValues {
				fVint, err2 := strconv.Atoi(filterValue)
				if err2 != nil {
					return false
				}
				if len(ctrIds) == fVint {
					return true
				}
			}
			return false
		}, nil
	case "ctr-status":
		for _, filterValue := range filterValues {
			if !util.StringInSlice(filterValue, []string{"created", "running", "paused", "stopped", "exited", "unknown"}) {
				return nil, errors.Errorf("%s is not a valid status", filterValue)
			}
		}
		return func(p *libpod.Pod) bool {
			ctrStatuses, err := p.Status()
			if err != nil {
				return false
			}
			for _, ctrStatus := range ctrStatuses {
				state := ctrStatus.String()
				if ctrStatus == define.ContainerStateConfigured {
					state = "created"
				} else if ctrStatus == define.ContainerStateStopped {
					state = "exited"
				}
				for _, filterValue := range filterValues {
					if filterValue == "stopped" {
						filterValue = "exited"
					}
					if state == filterValue {
						return true
					}
				}
			}
			return false
		}, nil
	case "id":
		return func(p *libpod.Pod) bool {
			return util.StringMatchRegexSlice(p.ID(), filterValues)
		}, nil
	case "name":
		return func(p *libpod.Pod) bool {
			return util.StringMatchRegexSlice(p.Name(), filterValues)
		}, nil
	case "status":
		for _, filterValue := range filterValues {
			if !util.StringInSlice(filterValue, []string{"stopped", "running", "paused", "exited", "dead", "created", "degraded"}) {
				return nil, errors.Errorf("%s is not a valid pod status", filterValue)
			}
		}
		return func(p *libpod.Pod) bool {
			status, err := p.GetPodStatus()
			if err != nil {
				return false
			}
			for _, filterValue := range filterValues {
				if strings.ToLower(status) == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "label":
		return func(p *libpod.Pod) bool {
			labels := p.Labels()
			return util.MatchLabelFilters(filterValues, labels)
		}, nil
	case "until":
		return func(p *libpod.Pod) bool {
			until, err := util.ComputeUntilTimestamp(filterValues)
			if err != nil {
				return false
			}
			if p.CreatedTime().Before(until) {
				return true
			}
			return false
		}, nil
	case "network":
		var inputNetNames []string
		for _, val := range filterValues {
			net, err := r.Network().NetworkInspect(val)
			if err != nil {
				// ignore not found errors
				break
			}
			inputNetNames = append(inputNetNames, net.Name)
		}
		return func(p *libpod.Pod) bool {
			infra, err := p.InfraContainer()
			// no infra, quick out
			if err != nil {
				return false
			}
			networks, err := infra.Networks()
			// if err or no networks, quick out
			if err != nil || len(networks) == 0 {
				return false
			}
			for _, net := range networks {
				if util.StringInSlice(net, inputNetNames) {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}
