package artifacts

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/bindings"
)

func Extract(ctx context.Context, artifactName string, targetDir string, options *ExtractOptions) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	if options == nil {
		options = new(ExtractOptions)
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

		// FIX: We should handle path traversal vulnerabilities here
		// by validating targetPath ends up being a sub dir of targetDir
		// after resolving "../" style links.
		targetPath := filepath.Join(targetDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(targetPath)
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
