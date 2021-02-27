package containers

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/copy"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
)

// Stat checks if the specified path is on the container.  Note that the stat
// report may be set even in case of an error.  This happens when the path
// resolves to symlink pointing to a non-existent path.
func Stat(ctx context.Context, nameOrID string, path string) (*entities.ContainerStatReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("path", path)

	response, err := conn.DoRequest(nil, http.MethodHead, "/containers/%s/archive", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}

	var finalErr error
	if response.StatusCode == http.StatusNotFound {
		finalErr = copy.ErrENOENT
	} else if response.StatusCode != http.StatusOK {
		finalErr = errors.New(response.Status)
	}

	var statReport *entities.ContainerStatReport

	fileInfo, err := copy.ExtractFileInfoFromHeader(&response.Header)
	if err != nil && finalErr == nil {
		return nil, err
	}

	if fileInfo != nil {
		statReport = &entities.ContainerStatReport{FileInfo: *fileInfo}
	}

	return statReport, finalErr
}

func CopyFromArchive(ctx context.Context, nameOrID string, path string, reader io.Reader) (entities.ContainerCopyFunc, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("path", path)

	return func() error {
		response, err := conn.DoRequest(reader, http.MethodPut, "/containers/%s/archive", params, nil, nameOrID)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			return errors.New(response.Status)
		}
		return response.Process(nil)
	}, nil
}

func CopyToArchive(ctx context.Context, nameOrID string, path string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("path", path)

	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/archive", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.Process(nil)
	}

	return func() error {
		_, err := io.Copy(writer, response.Body)
		return err
	}, nil
}
