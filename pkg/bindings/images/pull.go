package images

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

// Pull is the binding for libpod's v2 endpoints for pulling images.  Note that
// `rawImage` must be a reference to a registry (i.e., of docker transport or be
// normalized to one).  Other transports are rejected as they do not make sense
// in a remote context. Progress reported on stderr
func Pull(ctx context.Context, rawImage string, options *PullOptions) ([]string, error) {
	if options == nil {
		options = new(PullOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	params.Set("reference", rawImage)

	if options.SkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		// Note: we have to verify if skipped is false.
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.GetAuthfile(), options.GetUsername(), options.GetPassword())
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodPost, "/images/pull", params, header)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return nil, response.Process(err)
	}

	// Historically pull writes status to stderr
	stderr := io.Writer(os.Stderr)
	if options.GetQuiet() {
		stderr = ioutil.Discard
	}

	dec := json.NewDecoder(response.Body)
	var images []string
	var mErr error
	for {
		var report entities.ImagePullReport
		if err := dec.Decode(&report); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			report.Error = err.Error() + "\n"
		}

		select {
		case <-response.Request.Context().Done():
			return images, mErr
		default:
			// non-blocking select
		}

		switch {
		case report.Stream != "":
			fmt.Fprint(stderr, report.Stream)
		case report.Error != "":
			mErr = multierror.Append(mErr, errors.New(report.Error))
		case len(report.Images) > 0:
			images = report.Images
		case report.ID != "":
		default:
			return images, errors.Errorf("failed to parse pull results stream, unexpected input: %v", report)
		}
	}
	return images, mErr
}
