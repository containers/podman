package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgen/generate"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/pkg/errors"
)

// CreateContainer takes a specgenerator and makes a container. It returns
// the new container ID on success along with any warnings.
func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conf, err := runtime.GetConfigNoCopy()
	if err != nil {
		utils.InternalServerError(w, errors.Wrap(err, "failed to read podman config"))
		return
	}

	// we have to set the default before we decode to make sure the correct default is set when the field is unset
	sg := specgen.SpecGenerator{
		ContainerNetworkConfig: specgen.ContainerNetworkConfig{
			UseImageHosts: conf.Containers.NoHosts,
		},
	}

	if err := json.NewDecoder(r.Body).Decode(&sg); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if sg.Passwd == nil {
		t := true
		sg.Passwd = &t
	}

	// need to check for memory limit to adjust swap
	if sg.ResourceLimits != nil && sg.ResourceLimits.Memory != nil {
		s := ""
		var l int64
		if sg.ResourceLimits.Memory.Swap != nil {
			s = strconv.Itoa(int(*sg.ResourceLimits.Memory.Swap))
		}
		if sg.ResourceLimits.Memory.Limit != nil {
			l = *sg.ResourceLimits.Memory.Limit
		}
		specgenutil.LimitToSwap(sg.ResourceLimits.Memory, s, l)
	}

	warn, err := generate.CompleteSpec(r.Context(), runtime, &sg)
	if err != nil {
		utils.InternalServerError(w,
			errors.Wrapf(err, "failed to complete spec from provided body: %s", strings.Join(warn, "; ")))
		return
	}

	// Override any low-level Device fields, with filesystem specific fields if given
	// This provides backwards compatibility with previous API versions.
	for i, d := range sg.Devices {
		if len(d.PathOnHost) > 0 {
			spec, err := generate.DeviceFromPath(d.PathOnHost)
			if err != nil {
				utils.InternalServerError(w, errors.Wrapf(err, "failed to query linux device %q for container %q", d.Path, sg.Name))
				return
			}
			sg.Devices[i].FileMode = spec.FileMode
			sg.Devices[i].GID = spec.GID
			sg.Devices[i].Major = spec.Major
			sg.Devices[i].Minor = spec.Minor
			sg.Devices[i].Type = spec.Type
			sg.Devices[i].UID = spec.UID
			sg.Devices[i].PathOnHost = ""
		}

		if len(d.PathInContainer) > 0 {
			sg.Devices[i].Path = d.PathInContainer
			sg.Devices[i].PathInContainer = ""
		}

		// TODO Support 'm' - mknod permission... currently ignored
		if len(d.CgroupPermissions) > 0 {
			mode, err := generate.ParseFileMode(d.CgroupPermissions)
			if err != nil {
				utils.InternalServerError(w, fmt.Errorf("invalid device permission specification %q for container %q", d.CgroupPermissions, sg.Name))
				return
			}
			sg.Devices[i].FileMode = &mode
			sg.Devices[i].CgroupPermissions = ""
		}
	}

	rtSpec, spec, opts, err := generate.MakeContainer(context.Background(), runtime, &sg, false, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	ctr, err := generate.ExecuteCreate(context.Background(), runtime, rtSpec, spec, false, opts...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	response := entities.ContainerCreateResponse{ID: ctr.ID(), Warnings: warn}
	utils.WriteJSON(w, http.StatusCreated, response)
}
