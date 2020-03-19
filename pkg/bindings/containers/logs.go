package containers

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/libpod/pkg/bindings"
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
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/logs", params, nameOrID)
	if err != nil {
		return err
	}

	// read 8 bytes
	// first byte determines stderr=2|stdout=1
	// bytes 4-7 len(msg) in uint32
	for {
		stream, msgSize, err := readHeader(response.Body)
		if err != nil {
			// In case the server side closes up shop because !follow
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "unable to read log header")
		}
		msg, err := readMsg(response.Body, msgSize)
		if err != nil {
			return errors.Wrap(err, "unable to read log message")
		}
		if stream == 1 {
			stdoutChan <- msg
		} else {
			stderrChan <- msg
		}
	}
	return nil
}

func readMsg(r io.Reader, msgSize int) (string, error) {
	var msg []byte
	size := msgSize
	for {
		b := make([]byte, size)
		_, err := r.Read(b)
		if err != nil {
			return "", err
		}
		msg = append(msg, b...)
		if len(msg) == msgSize {
			break
		}
		size = msgSize - len(msg)
	}
	return string(msg), nil
}

func readHeader(r io.Reader) (byte, int, error) {
	var (
		header []byte
		size   = 8
	)
	for {
		b := make([]byte, size)
		_, err := r.Read(b)
		if err != nil {
			return 0, 0, err
		}
		header = append(header, b...)
		if len(header) == 8 {
			break
		}
		size = 8 - len(header)
	}
	stream := header[0]
	msgSize := int(binary.BigEndian.Uint32(header[4:]) - 8)
	return stream, msgSize, nil
}
