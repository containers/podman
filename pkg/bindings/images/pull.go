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

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/morikuni/aec"
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

	header, err := auth.MakeXRegistryAuthHeader(&types.SystemContext{AuthFilePath: options.GetAuthfile()}, options.GetUsername(), options.GetPassword())
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/images/pull", params, header)
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

	return parseImagePullReportStream(response.Body, stderr)
}

func clearLine(out io.Writer) {
	eraseMode := aec.EraseModes.All
	cl := aec.EraseLine(eraseMode)
	fmt.Fprint(out, cl)
}

func printStreamLine(report *entities.ImagePullReport, out io.Writer) error {
	var (
		endline    string
		statusLine string
	)

	if report.Error != "" {
		return errors.New(report.Error)
	}
	if report.Progress != nil {
		statusLine = report.Progress.String()
	} else {
		statusLine = report.Status
	}
	if statusLine != "" {
		clearLine(out)
		endline = "\r"
		fmt.Fprint(out, endline)
	}
	if statusLine != "" {
		fmt.Fprintf(out, "%s %s %s%s", report.Stream, report.ID, statusLine, endline)
	} else if report.Stream != "" {
		fmt.Fprintf(out, "%s", report.Stream)
	}
	return nil
}

func parseImagePullReportStream(in io.Reader, out io.Writer) ([]string, error) {
	var (
		images      []string
		parseErrors []error
		dec         = json.NewDecoder(in)
		ids         = make(map[string]uint)
	)
	for {
		var lines uint
		var report entities.ImagePullReport
		if err := dec.Decode(&report); err != nil {
			if err == io.EOF {
				break
			}
			return images, err
		}
		if report.ID != "" && (report.Progress != nil || report.Status != "") {
			line, ok := ids[report.ID]
			if !ok {
				line = uint(len(ids))
				ids[report.ID] = line
				fmt.Fprintf(out, "\n")
			}
			lines = uint(len(ids)) - line
			fmt.Fprint(out, aec.Up(lines))
		} else {
			ids = make(map[string]uint)
		}
		err := printStreamLine(&report, out)
		if report.ID != "" {
			fmt.Fprint(out, aec.Down(lines))
		}
		if len(report.Images) > 0 {
			images = report.Images
		}
		if err != nil {
			parseErrors = append(parseErrors, err)
		}
	}
	return images, errorhandling.JoinErrors(parseErrors)
}
