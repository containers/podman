package lpfilters

import (
	"strconv"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

// GeneratePodFilterFunc takes a filter and filtervalue (key, value)
// and generates a libpod function that can be used to filter
// pods
func GeneratePodFilterFunc(filter, filterValue string) (
	func(pod *libpod.Pod) bool, error) {
	switch filter {
	case "ctr-ids":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
			if err != nil {
				return false
			}
			return util.StringInSlice(filterValue, ctrIds)
		}, nil
	case "ctr-names":
		return func(p *libpod.Pod) bool {
			ctrs, err := p.AllContainers()
			if err != nil {
				return false
			}
			for _, ctr := range ctrs {
				if filterValue == ctr.Name() {
					return true
				}
			}
			return false
		}, nil
	case "ctr-number":
		return func(p *libpod.Pod) bool {
			ctrIds, err := p.AllContainersByID()
			if err != nil {
				return false
			}

			fVint, err2 := strconv.Atoi(filterValue)
			if err2 != nil {
				return false
			}
			return len(ctrIds) == fVint
		}, nil
	case "ctr-status":
		if !util.StringInSlice(filterValue,
			[]string{"created", "restarting", "running", "paused",
				"exited", "unknown"}) {
			return nil, errors.Errorf("%s is not a valid status", filterValue)
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
				}
				if state == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "id":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.ID(), filterValue)
		}, nil
	case "name":
		return func(p *libpod.Pod) bool {
			return strings.Contains(p.Name(), filterValue)
		}, nil
	case "status":
		if !util.StringInSlice(filterValue, []string{"stopped", "running", "paused", "exited", "dead", "created"}) {
			return nil, errors.Errorf("%s is not a valid pod status", filterValue)
		}
		return func(p *libpod.Pod) bool {
			status, err := p.GetPodStatus()
			if err != nil {
				return false
			}
			if strings.ToLower(status) == filterValue {
				return true
			}
			return false
		}, nil
	case "label":
		var filterArray = strings.SplitN(filterValue, "=", 2)
		var filterKey = filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(p *libpod.Pod) bool {
			for labelKey, labelValue := range p.Labels() {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}
