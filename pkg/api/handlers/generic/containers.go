package generic

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/containers/libpod/cmd/podman/shared"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/logs"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
		Link  bool `schema:"link"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if query.Link {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			utils.ErrLinkNotSupport)
		return
	}
	utils.RemoveContainer(w, r, query.Force, query.Vols)
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	containers, err := runtime.GetAllContainers()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	infoData, err := runtime.Info()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "Failed to obtain system info"))
		return
	}

	var list = make([]*handlers.Container, len(containers))
	for i, ctnr := range containers {
		api, err := handlers.LibpodToContainer(ctnr, infoData)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		list[i] = api
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	api, err := handlers.LibpodToContainerJSON(ctnr)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, api)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	con, err := utils.KillContainer(w, r)
	if err != nil {
		return
	}
	// the kill behavior for docker differs from podman in that they appear to wait
	// for the Container to croak so the exit code is accurate immediately after the
	// kill is sent.  libpod does not.  but we can add a wait here only for the docker
	// side of things and mimic that behavior
	if _, err = con.Wait(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to wait for Container %s", con.ID()))
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	var msg string
	// /{version}/containers/(name)/wait
	exitCode, err := utils.WaitContainer(w, r)
	if err != nil {
		msg = err.Error()
	}
	utils.WriteResponse(w, http.StatusOK, handlers.ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	var (
		decoder           = r.Context().Value("decoder").(*schema.Decoder)
		runtime           = r.Context().Value("runtime").(*libpod.Runtime)
		deletedContainers []string
		spaceReclaimed    uint64
		filterFuncs       []libpod.ContainerFilter
	)
	query := struct {
		Filters string `json:"filters"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	inputFilters := make(map[string][]string)
	if len(query.Filters) > 0 {
		if err := json.Unmarshal([]byte(query.Filters), &inputFilters); err != nil {
			utils.Error(w, "Failed to parse filters", http.StatusBadRequest, errors.Wrapf(err, "failed to parse filters"))
			return
		}
	}
	for key, values := range inputFilters {
		for _, val := range values {
			generatedFunc, err := shared.GenerateContainerFilterFuncs(key, val, runtime)
			if err != nil {
				utils.InternalServerError(w, errors.Wrapf(err, "failed to generate filter function"))
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}
	dcons, errs, err := runtime.PruneContainers(r.Context(), false, filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	for conID, sizeClaimed := range dcons {
		deletedContainers = append(deletedContainers, conID)
		spaceReclaimed += uint64(sizeClaimed)
	}
	for conID, err := range errs {
		log.Error(errors.Wrapf(err, "unable to prune %s", conID))
	}
	report := types.ContainersPruneReport{
		ContainersDeleted: deletedContainers,
		SpaceReclaimed:    spaceReclaimed,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func LogsFromContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Follow     bool   `schema:"follow"`
		Stdout     bool   `schema:"stdout"`
		Stderr     bool   `schema:"stderr"`
		Since      string `schema:"since"`
		Until      string `schema:"until"`
		Timestamps bool   `schema:"timestamps"`
		Tail       string `schema:"tail"`
	}{
		Tail: "all",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	if !(query.Stdout || query.Stderr) {
		msg := fmt.Sprintf("%s: you must choose at least one stream", http.StatusText(http.StatusBadRequest))
		utils.Error(w, msg, http.StatusBadRequest, errors.Errorf("%s for %s", msg, r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	var tail int64 = -1
	if query.Tail != "all" {
		tail, err = strconv.ParseInt(query.Tail, 0, 64)
		if err != nil {
			utils.BadRequest(w, "tail", query.Tail, err)
			return
		}
	}

	var since time.Time
	if _, found := mux.Vars(r)["since"]; found {
		since, err = util.ParseInputTime(query.Since)
		if err != nil {
			utils.BadRequest(w, "since", query.Since, err)
			return
		}
	}

	var until time.Time
	if _, found := mux.Vars(r)["until"]; found {
		since, err = util.ParseInputTime(query.Until)
		if err != nil {
			utils.BadRequest(w, "until", query.Until, err)
			return
		}
	}

	options := &logs.LogOptions{
		Details:    true,
		Follow:     query.Follow,
		Since:      since,
		Tail:       tail,
		Timestamps: query.Timestamps,
	}

	var wg sync.WaitGroup
	options.WaitGroup = &wg

	logChannel := make(chan *logs.LogLine, tail+1)
	if err := runtime.Log([]*libpod.Container{ctnr}, options, logChannel); err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "Failed to obtain logs for Container '%s'", name))
		return
	}
	go func() {
		wg.Wait()
		close(logChannel)
	}()

	w.WriteHeader(http.StatusOK)
	var builder strings.Builder
	for ok := true; ok; ok = query.Follow {
		for line := range logChannel {
			if _, found := mux.Vars(r)["until"]; found {
				if line.Time.After(until) {
					break
				}
			}

			// Reset variables we're ready to loop again
			builder.Reset()
			header := [8]byte{}

			switch line.Device {
			case "stdout":
				if !query.Stdout {
					continue
				}
				header[0] = 1
			case "stderr":
				if !query.Stderr {
					continue
				}
				header[0] = 2
			default:
				// Logging and moving on is the best we can do here. We may have already sent
				// a Status and Content-Type to client therefore we can no longer report an error.
				log.Infof("unknown Device type '%s' in log file from Container %s", line.Device, ctnr.ID())
				continue
			}

			if query.Timestamps {
				builder.WriteString(line.Time.Format(time.RFC3339))
				builder.WriteRune(' ')
			}
			builder.WriteString(line.Msg)

			// Build header and output entry
			binary.BigEndian.PutUint32(header[4:], uint32(len(header)+builder.Len()))
			if _, err := w.Write(header[:]); err != nil {
				log.Errorf("unable to write log output header: %q", err)
			}
			if _, err := fmt.Fprint(w, builder.String()); err != nil {
				log.Errorf("unable to write builder string: %q", err)
			}

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
