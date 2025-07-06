package artifacts

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/bindings"
)

func Extract(ctx context.Context, artifactName string, target string, options *ExtractOptions) error {
	var dest string

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	if options == nil {
		options = new(ExtractOptions)
	}

	// check if dest is a dir to know if we can copy more than one blob
	destIsFile := true
	stat, err := os.Stat(target)
	if err == nil {
		destIsFile = !stat.IsDir()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// ReviewNote: We could avoid this by checking tar after download but
	// it's usually better to fail early, rather than after transferring a 10g artifact for example

	// Inspect the Artifact to check layer count
	inspectReport, err := Inspect(ctx, artifactName, &InspectOptions{})
	if err != nil {
		return err
	}

	// If we're writing to a file, ensure only one blob is expected to be extracted and exclude returning unnecessary title.
	if destIsFile {
		if len(inspectReport.Manifest.Layers) > 1 {
			if options.Digest == nil && options.Title == nil {
				return fmt.Errorf("the artifact consists of several blobs and the target %q is not a directory and neither digest or title was specified to only copy a single blob", target)
			}
		}
		options.WithExcludeTitle(true)
	}

	params, err := options.ToParams()
	if err != nil {
		return err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/artifacts/%s/extract", params, nil, artifactName)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return response.Process(nil)
	}

	tr := tar.NewReader(response.Body)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		// If destination isn't a file, extract to dir/filename
		dest = target
		if !destIsFile {
			dest = filepath.Join(target, header.Name)
		}

		if header.Typeflag == tar.TypeReg {
			outFile, err := os.Create(dest)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				if err := outFile.Close(); err != nil {
					return fmt.Errorf("failed to close %s: %w", outFile.Name(), err)
				}

				return fmt.Errorf("failed to extract %s to %s: %w", header.Name, outFile.Name(), err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close %s: %w", outFile.Name(), err)
			}
		}
		// Note: Currently tar.TypeSymlink and tar.TypeLink are ignored
	}
	return nil
}
