//go:build !remote

package libpod

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/specgen/generate"
	"github.com/containers/podman/v5/pkg/specgenutil"
	"github.com/containers/storage"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// The JSON decoder correctly cannot decode (overflow) negative values (e.g., `-1`) for fields of type `uint64`,
// as `-1` is used to represent `max` in `POSIXRlimit`. To address this, we use `tmpSpecGenerator` to decode the request body.
// The `tmpSpecGenerator` overrides the `POSIXRlimit` type with a `tmpRlimit` type that uses the `json.Number` type for decoding values.
// The `tmpRlimit` is then parsed into the `POSIXRlimit` type and assigned to the `SpecGenerator`.
// This serves as a workaround for the issue (https://github.com/containers/podman/issues/24886).
type tmpSpecGenerator struct {
	specgen.SpecGenerator
	Rlimits []tmpRlimit `json:"r_limits,omitempty"`
}

type tmpRlimit struct {
	// Type of the rlimit to set
	Type string `json:"type"`
	// Hard is the hard limit for the specified type
	Hard json.Number `json:"hard"`
	// Soft is the soft limit for the specified type
	Soft json.Number `json:"soft"`
}

// CreateContainer takes a specgenerator and makes a container. It returns
// the new container ID on success along with any warnings.
func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conf, err := runtime.GetConfigNoCopy()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// copy vars here and not leak config pointers into specgen
	noHosts := conf.Containers.NoHosts
	privileged := conf.Containers.Privileged

	// we have to set the default before we decode to make sure the correct default is set when the field is unset
	tmpSg := tmpSpecGenerator{
		SpecGenerator: specgen.SpecGenerator{
			ContainerNetworkConfig: specgen.ContainerNetworkConfig{
				UseImageHosts: &noHosts,
			},
			ContainerSecurityConfig: specgen.ContainerSecurityConfig{
				Umask:      conf.Containers.Umask,
				Privileged: &privileged,
			},
			ContainerHealthCheckConfig: specgen.ContainerHealthCheckConfig{
				HealthLogDestination: define.DefaultHealthCheckLocalDestination,
				HealthMaxLogCount:    define.DefaultHealthMaxLogCount,
				HealthMaxLogSize:     define.DefaultHealthMaxLogSize,
			},
		},
	}

	if err := json.NewDecoder(r.Body).Decode(&tmpSg); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("decode(): %w", err))
		return
	}

	sg := tmpSg.SpecGenerator
	rLimits, err := parseRLimits(tmpSg.Rlimits)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid rlimit: %w", err))
		return
	}
	sg.Rlimits = rLimits

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
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.Error(w, http.StatusNotFound, fmt.Errorf("no such image: %w", err))
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	rtSpec, spec, opts, err := generate.MakeContainer(r.Context(), runtime, &sg, false, nil)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.Error(w, http.StatusNotFound, fmt.Errorf("no such image: %w", err))
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	ctr, err := generate.ExecuteCreate(r.Context(), runtime, rtSpec, spec, false, opts...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	response := entities.ContainerCreateResponse{ID: ctr.ID(), Warnings: warn}
	utils.WriteJSON(w, http.StatusCreated, response)
}

// parseRLimits parses slice of tmpLimit to slice of specs.POSIXRlimit.
func parseRLimits(rLimits []tmpRlimit) ([]specs.POSIXRlimit, error) {
	rl := []specs.POSIXRlimit{}

	// The "soft" and "hard" values are expected to be of type uint64.
	// The JSON decoder cannot cast -1 as to uint64.
	// We need to convert them to uint64, and handle the special case of -1
	// which indicates an max value.
	parseLimitNumber := func(limit json.Number) (uint64, error) {
		limitString := limit.String()
		if limitString == "-1" {
			// uint64(-1) overflow to max uint64 value
			return math.MaxUint64, nil
		}
		return strconv.ParseUint(limitString, 10, 64)
	}

	for _, rLimit := range rLimits {
		soft, err := parseLimitNumber(rLimit.Soft)
		if err != nil {
			return nil, fmt.Errorf("invalid value for POSIXRlimit.soft: %w", err)
		}
		hard, err := parseLimitNumber(rLimit.Hard)
		if err != nil {
			return nil, fmt.Errorf("invalid value for POSIXRlimit.hard: %w", err)
		}
		if rLimit.Type == "" {
			return nil, fmt.Errorf("invalid value for POSIXRlimit.type: %w", err)
		}

		rl = append(rl, specs.POSIXRlimit{
			Type: rLimit.Type,
			Soft: soft,
			Hard: hard,
		})
	}
	return rl, nil
}
