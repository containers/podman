package libpod

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/podman/v3/pkg/specgen/generate"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func PodCreate(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		err     error
	)
	var psg specgen.PodSpecGenerator
	if err := json.NewDecoder(r.Body).Decode(&psg); err != nil {
		utils.Error(w, "failed to decode specgen", http.StatusInternalServerError, errors.Wrap(err, "failed to decode specgen"))
		return
	}
	pod, err := generate.MakePod(&psg, runtime)
	if err != nil {
		httpCode := http.StatusInternalServerError
		if errors.Cause(err) == define.ErrPodExists {
			httpCode = http.StatusConflict
		}
		utils.Error(w, "Something went wrong.", httpCode, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, handlers.IDResponse{ID: pod.ID()})
}

func Pods(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	podPSOptions := entities.PodPSOptions{
		Filters: *filterMap,
	}
	pods, err := containerEngine.PodPs(r.Context(), podPSOptions)
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, pods)
}

func PodInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	podData, err := pod.Inspect()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	report := entities.PodInspectReport{
		InspectPodData: podData,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PodStop(w http.ResponseWriter, r *http.Request) {
	var (
		stopError error
		runtime   = r.Context().Value("runtime").(*libpod.Runtime)
		decoder   = r.Context().Value("decoder").(*schema.Decoder)
		responses map[string]error
	)
	query := struct {
		Timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	status, err := pod.GetPodStatus()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if status != define.PodStateRunning {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}

	if query.Timeout > 0 {
		responses, stopError = pod.StopWithTimeout(r.Context(), false, query.Timeout)
	} else {
		responses, stopError = pod.Stop(r.Context(), false)
	}
	if stopError != nil && errors.Cause(stopError) != define.ErrPodPartialFail {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	// Try to clean up the pod - but only warn on failure, it's nonfatal.
	if cleanupCtrs, cleanupErr := pod.Cleanup(r.Context()); cleanupErr != nil {
		logrus.Errorf("Error cleaning up pod %s: %v", pod.ID(), cleanupErr)
		for id, err := range cleanupCtrs {
			logrus.Errorf("Error cleaning up pod %s container %s: %v", pod.ID(), id, err)
		}
	}

	report := entities.PodStopReport{Id: pod.ID()}
	for id, err := range responses {
		report.Errs = append(report.Errs, errors.Wrapf(err, "error stopping container %s", id))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodStart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	status, err := pod.GetPodStatus()
	if err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	if status == define.PodStateRunning {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}

	responses, err := pod.Start(r.Context())
	if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
		utils.Error(w, "Something went wrong", http.StatusConflict, err)
		return
	}

	report := entities.PodStartReport{Id: pod.ID()}
	for id, err := range responses {
		report.Errs = append(report.Errs, errors.Wrapf(err, "error starting container "+id))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodDelete(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
	)
	query := struct {
		Force bool `schema:"force"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	if err := runtime.RemovePod(r.Context(), pod, true, query.Force); err != nil {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}
	report := entities.PodRmReport{Id: pod.ID()}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PodRestart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Restart(r.Context())
	if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	report := entities.PodRestartReport{Id: pod.ID()}
	for id, err := range responses {
		report.Errs = append(report.Errs, errors.Wrapf(err, "error restarting container %s", id))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodPrune(w http.ResponseWriter, r *http.Request) {
	reports, err := PodPruneHelper(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func PodPruneHelper(r *http.Request) ([]*entities.PodPruneReport, error) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
	)
	responses, err := runtime.PrunePods(r.Context())
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodPruneReport, 0, len(responses))
	for k, v := range responses {
		reports = append(reports, &entities.PodPruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func PodPause(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Pause(r.Context())
	if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
		utils.Error(w, "Something went wrong", http.StatusInternalServerError, err)
		return
	}

	report := entities.PodPauseReport{Id: pod.ID()}
	for id, v := range responses {
		report.Errs = append(report.Errs, errors.Wrapf(v, "error pausing container %s", id))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodUnpause(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Unpause(r.Context())
	if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
		utils.Error(w, "failed to pause pod", http.StatusInternalServerError, err)
		return
	}

	report := entities.PodUnpauseReport{Id: pod.ID()}
	for id, v := range responses {
		report.Errs = append(report.Errs, errors.Wrapf(v, "error unpausing container %s", id))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, &report)
}

func PodTop(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		PsArgs string `schema:"ps_args"`
	}{
		PsArgs: "",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	args := []string{}
	if query.PsArgs != "" {
		args = append(args, query.PsArgs)
	}
	output, err := pod.GetPodPidInformation(args)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	var body = handlers.PodTopOKBody{}
	if len(output) > 0 {
		body.Titles = strings.Split(output[0], "\t")
		for _, line := range output[1:] {
			body.Processes = append(body.Processes, strings.Split(line, "\t"))
		}
	}
	utils.WriteJSON(w, http.StatusOK, body)
}

func PodKill(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		decoder = r.Context().Value("decoder").(*schema.Decoder)
		signal  = "SIGKILL"
	)
	query := struct {
		Signal string `schema:"signal"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if _, found := r.URL.Query()["signal"]; found {
		signal = query.Signal
	}

	sig, err := util.ParseSignal(signal)
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "unable to parse signal value"))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	logrus.Debugf("Killing pod %s with signal %d", pod.ID(), sig)
	podStates, err := pod.Status()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	hasRunning := false
	for _, s := range podStates {
		if s == define.ContainerStateRunning {
			hasRunning = true
			break
		}
	}
	if !hasRunning {
		msg := fmt.Sprintf("Container %s is not running", pod.ID())
		utils.Error(w, msg, http.StatusConflict, errors.Errorf("cannot kill a pod with no running containers: %s", pod.ID()))
		return
	}

	responses, err := pod.Kill(r.Context(), uint(sig))
	if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
		utils.Error(w, "failed to kill pod", http.StatusInternalServerError, err)
		return
	}

	report := &entities.PodKillReport{Id: pod.ID()}
	for _, v := range responses {
		if v != nil {
			report.Errs = append(report.Errs, v)
		}
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	_, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PodStats(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		NamesOrIDs []string `schema:"namesOrIDs"`
		All        bool     `schema:"all"`
	}{
		// default would go here
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Validate input.
	options := entities.PodStatsOptions{All: query.All}
	if err := entities.ValidatePodStatsOptions(query.NamesOrIDs, &options); err != nil {
		utils.InternalServerError(w, err)
	}

	// Collect the stats and send them over the wire.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	reports, err := containerEngine.PodStats(r.Context(), query.NamesOrIDs, options)

	// Error checks as documented in swagger.
	switch errors.Cause(err) {
	case define.ErrNoSuchPod:
		utils.Error(w, "one or more pods not found", http.StatusNotFound, err)
		return
	case nil:
		// Nothing to do.
	default:
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, reports)
}
