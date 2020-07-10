package images

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/errorhandling"
)

// BachtRemove removes a batch of images from the local storage.
func BatchRemove(ctx context.Context, images []string, opts entities.ImageRemoveOptions) (*entities.ImageRemoveReport, []error) {
	// FIXME - bindings tests are missing for this endpoint. Once the CI is
	// re-enabled for bindings, we need to add them.  At the time of writing,
	// the tests don't compile.
	var report handlers.LibpodImagesRemoveReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, []error{err}
	}

	params := url.Values{}
	params.Set("all", strconv.FormatBool(opts.All))
	params.Set("force", strconv.FormatBool(opts.Force))
	for _, i := range images {
		params.Add("images", i)
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

// Remove removes an image from the local storage.  Use force to remove an
// image, even if it's used by containers.
func Remove(ctx context.Context, nameOrID string, force bool) (*entities.ImageRemoveReport, error) {
	var report handlers.LibpodImagesRemoveReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("force", strconv.FormatBool(force))
	response, err := conn.DoRequest(nil, http.MethodDelete, "/images/%s", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	if err := response.Process(&report); err != nil {
		return nil, err
	}

	errs := errorhandling.StringsToErrors(report.Errors)
	return &report.ImageRemoveReport, errorhandling.JoinErrors(errs)
}
