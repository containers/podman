package libpod

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	handlers.StopContainer(w, r)
}

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	_, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	utils.RemoveContainer(w, r, query.Force, query.Vols)
}
func ListContainers(w http.ResponseWriter, r *http.Request) {
	var (
		filterFuncs []libpod.ContainerFilter
		pss         []ListContainer
	)
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
	opts := shared.PsOptions{
		All:       query.All,
		Last:      query.Last,
		Size:      query.Size,
		Sort:      "",
		Namespace: query.Namespace,
		NoTrunc:   true,
		Pod:       query.Pod,
		Sync:      query.Sync,
	}
	if len(query.Filters) > 0 {
		for k, v := range query.Filters {
			for _, val := range v {
				generatedFunc, err := shared.GenerateContainerFilterFuncs(k, val, runtime)
				if err != nil {
					utils.InternalServerError(w, err)
					return
				}
				filterFuncs = append(filterFuncs, generatedFunc)
			}
		}
	}

	if !query.All {
		// The default is get only running containers. Do this with a filterfunc
		runningOnly, err := shared.GenerateContainerFilterFuncs("status", define.ContainerStateRunning.String(), runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, runningOnly)
	}

	cons, err := runtime.GetContainers(filterFuncs...)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	if query.Last > 0 {
		// Sort the containers we got
		sort.Sort(psSortCreateTime{cons})
		// we should perform the lopping before we start getting
		// the expensive information on containers
		if query.Last < len(cons) {
			cons = cons[len(cons)-query.Last:]
		}
	}
	for _, con := range cons {
		listCon, err := ListContainerBatch(runtime, con, opts)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		pss = append(pss, listCon)

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

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	_, err := utils.KillContainer(w, r)
	if err != nil {
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	exitCode, err := utils.WaitContainer(w, r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, strconv.Itoa(int(exitCode)))
}

func LogsFromContainer(w http.ResponseWriter, r *http.Request) {
	// follow
	// since
	// timestamps
	// tail string
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

// BatchContainerOp is used in ps to reduce performance hits by "batching"
// locks.
func ListContainerBatch(rt *libpod.Runtime, ctr *libpod.Container, opts shared.PsOptions) (ListContainer, error) {
	var (
		conConfig                               *libpod.ContainerConfig
		conState                                define.ContainerStatus
		err                                     error
		exitCode                                int32
		exited                                  bool
		pid                                     int
		size                                    *shared.ContainerSize
		startedTime                             time.Time
		exitedTime                              time.Time
		cgroup, ipc, mnt, net, pidns, user, uts string
	)

	batchErr := ctr.Batch(func(c *libpod.Container) error {
		conConfig = c.Config()
		conState, err = c.State()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container state")
		}

		exitCode, exited, err = c.ExitCode()
		if err != nil {
			return errors.Wrapf(err, "unable to obtain container exit code")
		}
		startedTime, err = c.StartedTime()
		if err != nil {
			logrus.Errorf("error getting started time for %q: %v", c.ID(), err)
		}
		exitedTime, err = c.FinishedTime()
		if err != nil {
			logrus.Errorf("error getting exited time for %q: %v", c.ID(), err)
		}

		if !opts.Size && !opts.Namespace {
			return nil
		}

		if opts.Namespace {
			pid, err = c.PID()
			if err != nil {
				return errors.Wrapf(err, "unable to obtain container pid")
			}
			ctrPID := strconv.Itoa(pid)
			cgroup, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "cgroup"))
			ipc, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "ipc"))
			mnt, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "mnt"))
			net, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "net"))
			pidns, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "pid"))
			user, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "user"))
			uts, _ = shared.GetNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "uts"))
		}
		if opts.Size {
			size = new(shared.ContainerSize)

			rootFsSize, err := c.RootFsSize()
			if err != nil {
				logrus.Errorf("error getting root fs size for %q: %v", c.ID(), err)
			}

			rwSize, err := c.RWSize()
			if err != nil {
				logrus.Errorf("error getting rw size for %q: %v", c.ID(), err)
			}

			size.RootFsSize = rootFsSize
			size.RwSize = rwSize
		}
		return nil
	})

	if batchErr != nil {
		return ListContainer{}, batchErr
	}

	ps := ListContainer{
		Command:   conConfig.Command,
		Created:   conConfig.CreatedTime.Unix(),
		Exited:    exited,
		ExitCode:  exitCode,
		ExitedAt:  exitedTime.Unix(),
		ID:        conConfig.ID,
		Image:     conConfig.RootfsImageName,
		IsInfra:   conConfig.IsInfra,
		Labels:    conConfig.Labels,
		Mounts:    ctr.UserVolumes(),
		Names:     []string{conConfig.Name},
		Pid:       pid,
		Pod:       conConfig.Pod,
		Ports:     conConfig.PortMappings,
		Size:      size,
		StartedAt: startedTime.Unix(),
		State:     conState.String(),
	}
	if opts.Pod && len(conConfig.Pod) > 0 {
		pod, err := rt.GetPod(conConfig.Pod)
		if err != nil {
			return ListContainer{}, err
		}
		ps.PodName = pod.Name()
	}

	if opts.Namespace {
		ns := ListContainerNamespaces{
			Cgroup: cgroup,
			IPC:    ipc,
			MNT:    mnt,
			NET:    net,
			PIDNS:  pidns,
			User:   user,
			UTS:    uts,
		}
		ps.Namespaces = ns
	}
	return ps, nil
}
