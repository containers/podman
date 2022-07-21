package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

// Push is the binding for libpod's endpoints for push images.  Note that
// `source` must be a referring to an image in the remote's container storage.
// The destination must be a reference to a registry (i.e., of docker transport
// or be normalized to one).  Other transports are rejected as they do not make
// sense in a remote context.
func Push(ctx context.Context, source string, destination string, options *PushOptions) error {
	if options == nil {
		options = new(PushOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	header, err := auth.MakeXRegistryAuthHeader(&imageTypes.SystemContext{AuthFilePath: options.GetAuthfile()}, options.GetUsername(), options.GetPassword())
	if err != nil {
		return err
	}

	params, err := options.ToParams()
	if err != nil {
		return err
	}
	// SkipTLSVerify is special.  We need to delete the param added by
	// toparams and change the key and flip the bool
	if options.SkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}
	params.Set("destination", destination)

	path := fmt.Sprintf("/images/%s/push", source)
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, path, params, header)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return response.Process(err)
	}

	// Historically push writes status to stderr
	writer := io.Writer(os.Stderr)
	if options.GetQuiet() {
		writer = ioutil.Discard
	}

	dec := json.NewDecoder(response.Body)
	for {
		var report entities.ImagePushReport
		if err := dec.Decode(&report); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		select {
		case <-response.Request.Context().Done():
			break
		default:
			// non-blocking select
		}

		switch {
		case report.Stream != "":
			fmt.Fprint(writer, report.Stream)
		case report.Error != "":
			// There can only be one error.
			return errors.New(report.Error)
		default:
			return fmt.Errorf("failed to parse push results stream, unexpected input: %v", report)
		}
	}

	return nil
}
