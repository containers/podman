package containers

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

// Checkpoint checkpoints the given container (identified by nameOrID).  All additional
// options are options and allow for more fine grained control of the checkpoint process.
func Checkpoint(ctx context.Context, nameOrID string, options *CheckpointOptions) (*entities.CheckpointReport, error) {
	var report entities.CheckpointReport
	if options == nil {
		options = new(CheckpointOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/checkpoint", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// Restore restores a checkpointed container to running. The container is identified by the nameOrID option. All
// additional options are optional and allow finer control of the restore process.
func Restore(ctx context.Context, nameOrID string, options *RestoreOptions) (*entities.RestoreReport, error) {
	var report entities.RestoreReport
	if options == nil {
		options = new(RestoreOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	// The import key is a reserved golang term
	params.Del("ImportArchive")
	if i := options.GetImportAchive(); options.Changed("ImportArchive") {
		params.Set("import", i)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/restore", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}
