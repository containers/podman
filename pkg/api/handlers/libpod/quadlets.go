//go:build !remote

package libpod

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.podman.io/storage/pkg/archive"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/handlers/utils"
	api "github.com/containers/podman/v6/pkg/api/types"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/domain/infra/abi"
	"github.com/containers/podman/v6/pkg/systemd/quadlet"
	"github.com/containers/podman/v6/pkg/util"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

func ListQuadlets(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(
			w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err),
		)
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadlets, err := containerEngine.QuadletList(r.Context(), entities.QuadletListOptions{Filters: filterMap})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, quadlets)
}

func GetQuadletPrint(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadletContents, err := containerEngine.QuadletPrint(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, fmt.Errorf("no such quadlet: %s: %w", name, err))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(quadletContents)); err != nil {
		logrus.Errorf("Failed to write quadlet contents: %v", err)
		return
	}
}

// validateQuadletContentType validates the Content-Type header for quadlet installation
func validateQuadletContentType(r *http.Request) (bool, error) {
	multipart := false
	if hdr, found := r.Header["Content-Type"]; found && len(hdr) > 0 {
		contentType, _, err := mime.ParseMediaType(hdr[0])
		if err != nil {
			return false, utils.GetBadRequestError("Content-Type", hdr[0], err)
		}

		switch contentType {
		case "application/tar":
			logrus.Infof("tar file content type is %s, should use \"application/x-tar\" content type", contentType)
		case "application/x-tar":
			break
		case "multipart/form-data":
			logrus.Infof("Received %s", hdr[0])
			multipart = true
		default:
			return false, utils.GetBadRequestError("Content-Type", hdr[0],
				fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\" or \"multipart/form-data\"", hdr[0]))
		}
	}
	return multipart, nil
}

// extractQuadletFiles extracts quadlet files from tar archive to a temporary directory
func extractQuadletFiles(tempDir string, r io.ReadCloser) ([]string, error) {
	quadletDir := filepath.Join(tempDir, "quadlets")
	err := os.Mkdir(quadletDir, 0o700)
	if err != nil {
		return nil, err
	}

	err = archive.Untar(r, quadletDir, nil)
	if err != nil {
		return nil, err
	}

	// Collect all files from the extracted directory
	var filePaths []string
	err = filepath.Walk(quadletDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	return filePaths, err
}

// processMultipartQuadlets processes multipart form data and saves files to temporary directory
func processMultipartQuadlets(tempDir string, r *http.Request) ([]string, error) {
	quadletDir := filepath.Join(tempDir, "quadlets")
	err := os.Mkdir(quadletDir, 0o700)
	if err != nil {
		return nil, err
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart reader: %w", err)
	}

	var filePaths []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read multipart: %w", err)
		}
		defer part.Close()

		filename := part.FileName()
		if filename == "" {
			// Skip parts without filenames
			continue
		}

		// Create file in temp directory
		filePath := filepath.Join(quadletDir, filename)
		file, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", filename, err)
		}
		defer file.Close()

		// Copy part content to file
		_, err = io.Copy(file, part)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", filename, err)
		}

		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// InstallQuadlets handles POST /libpod/quadlets to install quadlet files
func InstallQuadlets(w http.ResponseWriter, r *http.Request) {
	// Create temporary directory for processing
	contextDirectory, err := os.MkdirTemp("", "libpod_quadlet")
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(contextDirectory); err != nil {
			logrus.Warn(fmt.Errorf("failed to remove libpod_quadlet tmp directory %q: %w", contextDirectory, err))
		}
	}()

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	// Parse query parameters
	query := struct {
		Replace       bool `schema:"replace"`
		ReloadSystemd bool `schema:"reload-systemd"`
	}{
		Replace:       false,
		ReloadSystemd: true, // Default to true like CLI
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	multipart, err := validateQuadletContentType(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	var filePaths []string
	if multipart {
		logrus.Debug("Processing multipart form data")
		filePaths, err = processMultipartQuadlets(contextDirectory, r)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
	} else {
		logrus.Debug("Processing tar archive")
		filePaths, err = extractQuadletFiles(contextDirectory, r.Body)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}

	if len(filePaths) == 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("no files found in request"))
		return
	}

	if len(filePaths) > 0 {
		countQuadletFiles := 0
		for _, filePath := range filePaths {
			isQuadletFile := quadlet.IsExtSupported(filePath)
			if isQuadletFile {
				countQuadletFiles++
			}
		}
		if countQuadletFiles > 1 {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("only a single quadlet file is allowed per request"))
			return
		}
		if countQuadletFiles == 0 {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("no quadlet files found in request"))
			return
		}
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	installOptions := entities.QuadletInstallOptions{
		Replace:       query.Replace,
		ReloadSystemd: query.ReloadSystemd,
	}

	installReport, err := containerEngine.QuadletInstall(r.Context(), filePaths, installOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(installReport.QuadletErrors) > 0 {
		errorStrings := make([]string, 0, len(installReport.QuadletErrors))
		for path, err := range installReport.QuadletErrors {
			errorStrings = append(errorStrings, fmt.Sprintf("%s: %v", path, err.Error()))
		}
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("errors occurred installing some Quadlets: %s", strings.Join(errorStrings, ", ")))
		return
	}

	utils.WriteResponse(w, http.StatusOK, installReport)
}
