package libpod

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/compat"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	name := utils.GetName(r)
	query := struct {
		External bool `schema:"external"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.ContainerExistsOptions{
		External: query.External,
	}

	report, err := containerEngine.ContainerExists(r.Context(), name, options)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if report.Value {
		utils.WriteResponse(w, http.StatusNoContent, "")
	} else {
		utils.ContainerNotFound(w, name, define.ErrNoSuchCtr)
	}
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All       bool `schema:"all"`
		External  bool `schema:"external"`
		Last      int  `schema:"last"` // alias for limit
		Limit     int  `schema:"limit"`
		Namespace bool `schema:"namespace"`
		Size      bool `schema:"size"`
		Sync      bool `schema:"sync"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to decode filter parameters for %s", r.URL.String()))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	limit := query.Limit
	// Support `last` as an alias for `limit`.  While Podman uses --last in
	// the CLI, the API is using `limit`.  As we first used `last` in the
	// API as well, we decided to go with aliasing to prevent any
	// regression. See github.com/containers/podman/issues/6413.
	if _, found := r.URL.Query()["last"]; found {
		logrus.Info("List containers: received `last` parameter - overwriting `limit`")
		limit = query.Last
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	opts := entities.ContainerListOptions{
		All:       query.All,
		External:  query.External,
		Filters:   *filterMap,
		Last:      limit,
		Namespace: query.Namespace,
		// Always return Pod, should not be part of the API.
		// https://github.com/containers/podman/pull/7223
		Pod:  true,
		Size: query.Size,
		Sync: query.Sync,
	}
	pss, err := containerEngine.ContainerList(r.Context(), opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(pss) == 0 {
		utils.WriteResponse(w, http.StatusOK, "[]")
		return
	}
	utils.WriteResponse(w, http.StatusOK, pss)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	container, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	data, err := container.Inspect(query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, data)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	utils.WaitContainerLibpod(w, r)
}

func UnmountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	conn, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	// TODO In future it might be an improvement that libpod unmount return a
	// "container not mounted" error so we can surface that to the endpoint user
	if err := conn.Unmount(false); err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
func MountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	conn, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	m, err := conn.Mount()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, m)
}

func ShowMountedContainers(w http.ResponseWriter, r *http.Request) {
	response := make(map[string]string)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conns, err := runtime.GetAllContainers()
	if err != nil {
		utils.InternalServerError(w, err)
	}
	for _, conn := range conns {
		mounted, mountPoint, err := conn.Mounted()
		if err != nil {
			utils.InternalServerError(w, err)
		}
		if !mounted {
			continue
		}
		response[conn.ID()] = mountPoint
	}
	utils.WriteResponse(w, http.StatusOK, response)
}

func Checkpoint(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Keep           bool `schema:"keep"`
		LeaveRunning   bool `schema:"leaveRunning"`
		TCPEstablished bool `schema:"tcpEstablished"`
		Export         bool `schema:"export"`
		IgnoreRootFS   bool `schema:"ignoreRootFS"`
		PrintStats     bool `schema:"printStats"`
		PreCheckpoint  bool `schema:"preCheckpoint"`
		WithPrevious   bool `schema:"withPrevious"`
		FileLocks      bool `schema:"fileLocks"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := utils.GetName(r)
	if _, err := runtime.LookupContainer(name); err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	names := []string{name}

	options := entities.CheckpointOptions{
		Keep:           query.Keep,
		LeaveRunning:   query.LeaveRunning,
		TCPEstablished: query.TCPEstablished,
		IgnoreRootFS:   query.IgnoreRootFS,
		PrintStats:     query.PrintStats,
		PreCheckPoint:  query.PreCheckpoint,
		WithPrevious:   query.WithPrevious,
		FileLocks:      query.FileLocks,
	}

	if query.Export {
		f, err := ioutil.TempFile("", "checkpoint")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer os.Remove(f.Name())
		if err := f.Close(); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		options.Export = f.Name()
	}

	reports, err := containerEngine.ContainerCheckpoint(r.Context(), names, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	if !query.Export {
		if len(reports) != 1 {
			utils.InternalServerError(w, fmt.Errorf("expected 1 restore report but got %d", len(reports)))
			return
		}
		if reports[0].Err != nil {
			utils.InternalServerError(w, reports[0].Err)
			return
		}
		utils.WriteResponse(w, http.StatusOK, reports[0])
		return
	}

	f, err := os.Open(options.Export)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer f.Close()
	utils.WriteResponse(w, http.StatusOK, f)
}

func Restore(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Keep            bool   `schema:"keep"`
		TCPEstablished  bool   `schema:"tcpEstablished"`
		Import          bool   `schema:"import"`
		Name            string `schema:"name"`
		IgnoreRootFS    bool   `schema:"ignoreRootFS"`
		IgnoreVolumes   bool   `schema:"ignoreVolumes"`
		IgnoreStaticIP  bool   `schema:"ignoreStaticIP"`
		IgnoreStaticMAC bool   `schema:"ignoreStaticMAC"`
		PrintStats      bool   `schema:"printStats"`
		FileLocks       bool   `schema:"fileLocks"`
		PublishPorts    string `schema:"publishPorts"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := entities.RestoreOptions{
		Name:            query.Name,
		Keep:            query.Keep,
		TCPEstablished:  query.TCPEstablished,
		IgnoreRootFS:    query.IgnoreRootFS,
		IgnoreVolumes:   query.IgnoreVolumes,
		IgnoreStaticIP:  query.IgnoreStaticIP,
		IgnoreStaticMAC: query.IgnoreStaticMAC,
		PrintStats:      query.PrintStats,
		FileLocks:       query.FileLocks,
		PublishPorts:    strings.Fields(query.PublishPorts),
	}

	var names []string
	if query.Import {
		t, err := ioutil.TempFile("", "restore")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer os.Remove(t.Name())
		if err := compat.SaveFromBody(t, r); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		options.Import = t.Name()
	} else {
		name := utils.GetName(r)
		if _, err := runtime.LookupContainer(name); err != nil {
			utils.ContainerNotFound(w, name, err)
			return
		}
		names = []string{name}
	}

	reports, err := containerEngine.ContainerRestore(r.Context(), names, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(reports) != 1 {
		utils.InternalServerError(w, fmt.Errorf("expected 1 restore report but got %d", len(reports)))
		return
	}
	if reports[0].Err != nil {
		utils.InternalServerError(w, reports[0].Err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports[0])
}

func InitContainer(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	err = ctr.Init(r.Context(), ctr.PodID() != "")
	if errors.Cause(err) == define.ErrCtrStateInvalid {
		utils.Error(w, http.StatusNotModified, err)
		return
	}
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ShouldRestart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	name := utils.GetName(r)
	report, err := containerEngine.ShouldRestart(r.Context(), name)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if report.Value {
		utils.WriteResponse(w, http.StatusNoContent, "")
	} else {
		utils.ContainerNotFound(w, name, define.ErrNoSuchCtr)
	}
}
