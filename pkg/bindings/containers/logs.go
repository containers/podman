package containers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/pkg/errors"
)

// Logs obtains a container's logs given the options provided.  The logs are then sent to the
// stdout|stderr channels as strings.
func Logs(ctx context.Context, nameOrID string, opts LogOptions, stdoutChan, stderrChan chan string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if opts.Follow != nil {
		params.Set("follow", strconv.FormatBool(*opts.Follow))
	}
	if opts.Since != nil {
		params.Set("since", *opts.Since)
	}
	if opts.Stderr != nil {
		params.Set("stderr", strconv.FormatBool(*opts.Stderr))
	}
	if opts.Stdout != nil {
		params.Set("stdout", strconv.FormatBool(*opts.Stdout))
	}
	if opts.Tail != nil {
		params.Set("tail", *opts.Tail)
	}
	if opts.Timestamps != nil {
		params.Set("timestamps", strconv.FormatBool(*opts.Timestamps))
	}
	if opts.Until != nil {
		params.Set("until", *opts.Until)
	}
	// The API requires either stdout|stderr be used. If neither are specified, we specify stdout
	if opts.Stdout == nil && opts.Stderr == nil {
		params.Set("stdout", strconv.FormatBool(true))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/logs", params, nil, nameOrID)
	if err != nil {
		return err
	}

	buffer := make([]byte, 1024)
	for {
		fd, l, err := DemuxHeader(response.Body, buffer)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		frame, err := DemuxFrame(response.Body, buffer, l)
		if err != nil {
			return err
		}

		switch fd {
		case 0:
			stdoutChan <- string(frame)
		case 1:
			stdoutChan <- string(frame)
		case 2:
			stderrChan <- string(frame)
		case 3:
			return errors.New("error from service in stream: " + string(frame))
		default:
			return fmt.Errorf("unrecognized input header: %d", fd)
		}
	}
}
