package lpfilters

import (
	"regexp"
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
func GenerateContainerFilterFuncs(filter, filterValue string, r *libpod.Runtime) (func(container *libpod.Container) bool, error) {
	switch filter {
	case "id":
		return func(c *libpod.Container) bool {
			return strings.Contains(c.ID(), filterValue)
		}, nil
	case "label":
		var filterArray = strings.SplitN(filterValue, "=", 2)
		var filterKey = filterArray[0]
		if len(filterArray) > 1 {
			filterValue = filterArray[1]
		} else {
			filterValue = ""
		}
		return func(c *libpod.Container) bool {
			for labelKey, labelValue := range c.Labels() {
				if labelKey == filterKey && ("" == filterValue || labelValue == filterValue) {
					return true
				}
			}
			return false
		}, nil
	case "name":
		return func(c *libpod.Container) bool {
			match, err := regexp.MatchString(filterValue, c.Name())
			if err != nil {
				return false
			}
			return match
		}, nil
	case "exited":
		exitCode, err := strconv.ParseInt(filterValue, 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "exited code out of range %q", filterValue)
		}
		return func(c *libpod.Container) bool {
			ec, exited, err := c.ExitCode()
			if ec == int32(exitCode) && err == nil && exited {
				return true
			}
			return false
		}, nil
	case "status":
		if !util.StringInSlice(filterValue, []string{"created", "running", "paused", "stopped", "exited", "unknown"}) {
			return nil, errors.Errorf("%s is not a valid status", filterValue)
		}
		return func(c *libpod.Container) bool {
			status, err := c.State()
			if err != nil {
				return false
			}
			if filterValue == "stopped" {
				filterValue = "exited"
			}
			state := status.String()
			if status == define.ContainerStateConfigured {
				state = "created"
			} else if status == define.ContainerStateStopped {
				state = "exited"
			}
			return state == filterValue
		}, nil
	case "ancestor":
		// This needs to refine to match docker
		// - ancestor=(<image-name>[:tag]|<image-id>| ⟨image@digest⟩) - containers created from an image or a descendant.
		return func(c *libpod.Container) bool {
			containerConfig := c.Config()
			if strings.Contains(containerConfig.RootfsImageID, filterValue) || strings.Contains(containerConfig.RootfsImageName, filterValue) {
				return true
			}
			return false
		}, nil
	case "before":
		ctr, err := r.LookupContainer(filterValue)
		if err != nil {
			return nil, errors.Errorf("unable to find container by name or id of %s", filterValue)
		}
		containerConfig := ctr.Config()
		createTime := containerConfig.CreatedTime
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.After(cc.CreatedTime)
		}, nil
	case "since":
		ctr, err := r.LookupContainer(filterValue)
		if err != nil {
			return nil, errors.Errorf("unable to find container by name or id of %s", filterValue)
		}
		containerConfig := ctr.Config()
		createTime := containerConfig.CreatedTime
		return func(c *libpod.Container) bool {
			cc := c.Config()
			return createTime.Before(cc.CreatedTime)
		}, nil
	case "volume":
		//- volume=(<volume-name>|<mount-point-destination>)
		return func(c *libpod.Container) bool {
			containerConfig := c.Config()
			var dest string
			arr := strings.Split(filterValue, ":")
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
			return false
		}, nil
	case "health":
		return func(c *libpod.Container) bool {
			hcStatus, err := c.HealthCheckStatus()
			if err != nil {
				return false
			}
			return hcStatus == filterValue
		}, nil
	case "until":
		ts, err := timetype.GetTimestamp(filterValue, time.Now())
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
