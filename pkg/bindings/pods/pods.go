package pods

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	jsoniter "github.com/json-iterator/go"
)

func CreatePodFromSpec(ctx context.Context, spec *entities.PodSpec) (*entities.PodCreateReport, error) {
	var (
		pcr entities.PodCreateReport
	)
	if spec == nil {
		spec = new(entities.PodSpec)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	specString, err := jsoniter.MarshalToString(spec.PodSpecGen)
	if err != nil {
		return nil, err
	}
	stringReader := strings.NewReader(specString)
	response, err := conn.DoRequest(ctx, stringReader, http.MethodPost, "/pods/create", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &pcr, response.Process(&pcr)
}

// Exists is a lightweight method to determine if a pod exists in local storage
func Exists(ctx context.Context, nameOrID string, options *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/pods/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}

// Inspect returns low-level information about the given pod.
func Inspect(ctx context.Context, nameOrID string, options *InspectOptions) (*entities.PodInspectReport, error) {
	var (
		report entities.PodInspectReport
	)
	if options == nil {
		options = new(InspectOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/pods/%s/json", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}

// Kill sends a SIGTERM to all the containers in a pod.  The optional signal parameter
// can be used to override  SIGTERM.
func Kill(ctx context.Context, nameOrID string, options *KillOptions) (*entities.PodKillReport, error) {
	var (
		report entities.PodKillReport
	)
	if options == nil {
		options = new(KillOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/kill", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Pause pauses all running containers in a given pod.
func Pause(ctx context.Context, nameOrID string, options *PauseOptions) (*entities.PodPauseReport, error) {
	var report entities.PodPauseReport
	if options == nil {
		options = new(PauseOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/pause", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Prune by default removes all non-running pods in local storage.
// And with force set true removes all pods.
func Prune(ctx context.Context, options *PruneOptions) ([]*entities.PodPruneReport, error) {
	var reports []*entities.PodPruneReport
	if options == nil {
		options = new(PruneOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/prune", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return reports, response.Process(&reports)
}

// List returns all pods in local storage.  The optional filters parameter can
// be used to refine which pods should be listed.
func List(ctx context.Context, options *ListOptions) ([]*entities.ListPodsReport, error) {
	var (
		podsReports []*entities.ListPodsReport
	)
	if options == nil {
		options = new(ListOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/pods/json", params, nil)
	if err != nil {
		return podsReports, err
	}
	defer response.Body.Close()

	return podsReports, response.Process(&podsReports)
}

// Restart restarts all containers in a pod.
func Restart(ctx context.Context, nameOrID string, options *RestartOptions) (*entities.PodRestartReport, error) {
	var report entities.PodRestartReport
	if options == nil {
		options = new(RestartOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/restart", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Remove deletes a Pod from from local storage. The optional force parameter denotes
// that the Pod can be removed even if in a running state.
func Remove(ctx context.Context, nameOrID string, options *RemoveOptions) (*entities.PodRmReport, error) {
	var report entities.PodRmReport
	if options == nil {
		options = new(RemoveOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/pods/%s", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}

// Start starts all containers in a pod.
func Start(ctx context.Context, nameOrID string, options *StartOptions) (*entities.PodStartReport, error) {
	var report entities.PodStartReport
	if options == nil {
		options = new(StartOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/start", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		report.Id = nameOrID
		return &report, nil
	}

	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Stop stops all containers in a Pod. The optional timeout parameter can be
// used to override the timeout before the container is killed.
func Stop(ctx context.Context, nameOrID string, options *StopOptions) (*entities.PodStopReport, error) {
	var report entities.PodStopReport
	if options == nil {
		options = new(StopOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/stop", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		report.Id = nameOrID
		return &report, nil
	}
	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Top gathers statistics about the running processes in a pod. The nameOrID can be a pod name
// or a partial/full ID.  The descriptors allow for specifying which data to collect from each process.
func Top(ctx context.Context, nameOrID string, options *TopOptions) ([]string, error) {
	if options == nil {
		options = new(TopOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if descriptors := options.GetDescriptors(); len(descriptors) > 0 {
		params.Set("ps_args", strings.Join(descriptors, ","))
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/pods/%s/top", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body := handlers.PodTopOKBody{}
	if err = response.Process(&body); err != nil {
		return nil, err
	}

	// handlers.PodTopOKBody{} returns a slice of slices where each cell in the top table is an item.
	// In libpod land, we're just using a slice with cells being split by tabs, which allows for an idiomatic
	// usage of the tabwriter.
	topOutput := []string{strings.Join(body.Titles, "\t")}
	for _, out := range body.Processes {
		topOutput = append(topOutput, strings.Join(out, "\t"))
	}

	return topOutput, err
}

// Unpause unpauses all paused containers in a Pod.
func Unpause(ctx context.Context, nameOrID string, options *UnpauseOptions) (*entities.PodUnpauseReport, error) {
	if options == nil {
		options = new(UnpauseOptions)
	}
	_ = options
	var report entities.PodUnpauseReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/pods/%s/unpause", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.ProcessWithError(&report, &errorhandling.PodConflictErrorModel{})
}

// Stats display resource-usage statistics of one or more pods.
func Stats(ctx context.Context, namesOrIDs []string, options *StatsOptions) ([]*entities.PodStatsReport, error) {
	if options == nil {
		options = new(StatsOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	for _, i := range namesOrIDs {
		params.Add("namesOrIDs", i)
	}

	var reports []*entities.PodStatsReport
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/pods/stats", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return reports, response.Process(&reports)
}
