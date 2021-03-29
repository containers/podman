package images

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/pkg/api/handlers/types"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/errorhandling"
)

// Remove removes one or more images from the local storage.  Use optional force option to remove an
// image, even if it's used by containers.
func Remove(ctx context.Context, images []string, options *RemoveOptions) (*entities.ImageRemoveReport, []error) {
	if options == nil {
		options = new(RemoveOptions)
	}
	// FIXME - bindings tests are missing for this endpoint. Once the CI is
	// re-enabled for bindings, we need to add them.  At the time of writing,
	// the tests don't compile.
	var report types.LibpodImagesRemoveReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, []error{err}
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, nil
	}
	for _, image := range images {
		params.Add("images", image)
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/images/remove", params, nil)
	if err != nil {
		return nil, []error{err}
	}
	if err := response.Process(&report); err != nil {
		return nil, []error{err}
	}

	return &report.ImageRemoveReport, errorhandling.StringsToErrors(report.Errors)
}
