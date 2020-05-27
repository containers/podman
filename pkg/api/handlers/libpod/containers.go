package libpod

import (
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/ps"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	_, err := runtime.LookupContainer(name)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			utils.ContainerNotFound(w, name, err)
		}
		utils.InternalServerError(w, err)
		return

	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All       bool                `schema:"all"`
		Filters   map[string][]string `schema:"filters"`
		Last      int                 `schema:"last"`
		Namespace bool                `schema:"namespace"`
		Pod       bool                `schema:"pod"`
		Size      bool                `schema:"size"`
		Sync      bool                `schema:"sync"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	opts := entities.ContainerListOptions{
		All:       query.All,
		Filters:   query.Filters,
		Last:      query.Last,
		Size:      query.Size,
		Sort:      "",
		Namespace: query.Namespace,
		Pod:       query.Pod,
		Sync:      query.Sync,
	}
	pss, err := ps.GetContainerLists(runtime, opts)
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
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
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
	exitCode, err := utils.WaitContainer(w, r)
	if err != nil {
		return
	}
	utils.WriteResponse(w, http.StatusOK, strconv.Itoa(int(exitCode)))
}

func UnmountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
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
	var targetFile string
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Keep           bool `schema:"keep"`
		LeaveRunning   bool `schema:"leaveRunning"`
		TCPEstablished bool `schema:"tcpEstablished"`
		Export         bool `schema:"export"`
		IgnoreRootFS   bool `schema:"ignoreRootFS"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	if query.Export {
		tmpFile, err := ioutil.TempFile("", "checkpoint")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer os.Remove(tmpFile.Name())
		if err := tmpFile.Close(); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		targetFile = tmpFile.Name()
	}
	options := libpod.ContainerCheckpointOptions{
		Keep:           query.Keep,
		KeepRunning:    query.LeaveRunning,
		TCPEstablished: query.TCPEstablished,
		IgnoreRootfs:   query.IgnoreRootFS,
	}
	if query.Export {
		options.TargetFile = targetFile
	}
	err = ctr.Checkpoint(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if query.Export {
		f, err := os.Open(targetFile)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer f.Close()
		utils.WriteResponse(w, http.StatusOK, f)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.CheckpointReport{Id: ctr.ID()})
}

func Restore(w http.ResponseWriter, r *http.Request) {
	var (
		targetFile string
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Keep            bool   `schema:"keep"`
		TCPEstablished  bool   `schema:"tcpEstablished"`
		Import          bool   `schema:"import"`
		Name            string `schema:"name"`
		IgnoreRootFS    bool   `schema:"ignoreRootFS"`
		IgnoreStaticIP  bool   `schema:"ignoreStaticIP"`
		IgnoreStaticMAC bool   `schema:"ignoreStaticMAC"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	if query.Import {
		t, err := ioutil.TempFile("", "restore")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer t.Close()
		if err := compat.SaveFromBody(t, r); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		targetFile = t.Name()
	}

	options := libpod.ContainerCheckpointOptions{
		Keep:            query.Keep,
		TCPEstablished:  query.TCPEstablished,
		IgnoreRootfs:    query.IgnoreRootFS,
		IgnoreStaticIP:  query.IgnoreStaticIP,
		IgnoreStaticMAC: query.IgnoreStaticMAC,
	}
	if query.Import {
		options.TargetFile = targetFile
		options.Name = query.Name
	}
	err = ctr.Restore(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.RestoreReport{Id: ctr.ID()})
}

func InitContainer(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	err = ctr.Init(r.Context())
	if errors.Cause(err) == define.ErrCtrStateInvalid {
		utils.Error(w, "container already initialized", http.StatusNotModified, err)
		return
	}
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
