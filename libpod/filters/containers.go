package lpfilters

import (
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/timetype"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

// GenerateContainerFilterFuncs return ContainerFilter functions based of filter.
func GenerateContainerFilterFuncs(filter string, filterValues []string, r *libpod.Runtime) (func(container *libpod.Container) bool, error) {
	switch filter {
	case "id":
		// we only have to match one ID
		return func(c *libpod.Container) bool {
			return util.StringMatchRegexSlice(c.ID(), filterValues)
		}, nil
	case "label":
		// we have to match that all given labels exits on that container
		return func(c *libpod.Container) bool {
			labels := c.Labels()
			for _, filterValue := range filterValues {
				matched := false
				filterArray := strings.SplitN(filterValue, "=", 2)
				filterKey := filterArray[0]
				if len(filterArray) > 1 {
					filterValue = filterArray[1]
				} else {
					filterValue = ""
				}
				for labelKey, labelValue := range labels {
					if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
						matched = true
						break
					}
				}
				if !matched {
					return false
				}
			}
			return true
		}, nil
	case "name":
		// we only have to match one name
		return func(c *libpod.Container) bool {
			return util.StringMatchRegexSlice(c.Name(), filterValues)
		}, nil
	case "exited":
		var exitCodes []int32
		for _, exitCode := range filterValues {
			ec, err := strconv.ParseInt(exitCode, 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "exited code out of range %q", ec)
			}
			exitCodes = append(exitCodes, int32(ec))
		}
		return func(c *libpod.Container) bool {
			ec, exited, err := c.ExitCode()
			if err == nil && exited {
				for _, exitCode := range exitCodes {
					if ec == exitCode {
						return true
					}
				}
			}
			return false
		}, nil
	case "status":
		for _, filterValue := range filterValues {
			if !util.StringInSlice(filterValue, []string{"created", "running", "paused", "stopped", "exited", "unknown"}) {
				return nil, errors.Errorf("%s is not a valid status", filterValue)
			}
		}
		return func(c *libpod.Container) bool {
			status, err := c.State()
			if err != nil {
				return false
			}
			state := status.String()
			if status == define.ContainerStateConfigured {
				state = "created"
			} else if status == define.ContainerStateStopped {
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
			return false
		}, nil
	case "ancestor":
		// This needs to refine to match docker
		// - ancestor=(<image-name>[:tag]|<image-id>| ⟨image@digest⟩) - containers created from an image or a descendant.
		return func(c *libpod.Container) bool {
			for _, filterValue := range filterValues {
				containerConfig := c.Config()
				if strings.Contains(containerConfig.RootfsImageID, filterValue) || strings.Contains(containerConfig.RootfsImageName, filterValue) {
					return true
				}
			}
			return false
		}, nil
	case "before":
		var createTime time.Time
		for _, filterValue := range filterValues {
			ctr, err := r.LookupContainer(filterValue)
			if err != nil {
				return nil, err
			}
			containerConfig := ctr.Config()
			if createTime.IsZero() || createTime.After(containerConfig.CreatedTime) {
				createTime = containerConfig.CreatedTime
			}
		}
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.After(cc.CreatedTime)
		}, nil
	case "since":
		var createTime time.Time
		for _, filterValue := range filterValues {
			ctr, err := r.LookupContainer(filterValue)
			if err != nil {
				return nil, err
			}
			containerConfig := ctr.Config()
			if createTime.IsZero() || createTime.After(containerConfig.CreatedTime) {
				createTime = containerConfig.CreatedTime
			}
		}
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.Before(cc.CreatedTime)
		}, nil
	case "volume":
		//- volume=(<volume-name>|<mount-point-destination>)
		return func(c *libpod.Container) bool {
			containerConfig := c.Config()
			var dest string
			for _, filterValue := range filterValues {
				arr := strings.SplitN(filterValue, ":", 2)
				source := arr[0]
				if len(arr) == 2 {
					dest = arr[1]
				}
				for _, mount := range containerConfig.Spec.Mounts {
					if dest != "" && (mount.Source == source && mount.Destination == dest) {
						return true
					}
					if dest == "" && mount.Source == source {
						return true
					}
				}
				for _, vname := range containerConfig.NamedVolumes {
					if dest != "" && (vname.Name == source && vname.Dest == dest) {
						return true
					}
					if dest == "" && vname.Name == source {
						return true
					}
				}
			}
			return false
		}, nil
	case "health":
		return func(c *libpod.Container) bool {
			hcStatus, err := c.HealthCheckStatus()
			if err != nil {
				return false
			}
			for _, filterValue := range filterValues {
				if hcStatus == filterValue {
					return true
				}
			}
			return false
		}, nil
	case "until":
		if len(filterValues) != 1 {
			return nil, errors.Errorf("specify exactly one timestamp for %s", filter)
		}
		ts, err := timetype.GetTimestamp(filterValues[0], time.Now())
		if err != nil {
			return nil, err
		}
		seconds, nanoseconds, err := timetype.ParseTimestamps(ts, 0)
		if err != nil {
			return nil, err
		}
		until := time.Unix(seconds, nanoseconds)
		return func(c *libpod.Container) bool {
			if !until.IsZero() && c.CreatedTime().After((until)) {
				return true
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}
