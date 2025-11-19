//go:build !remote

package server

import (
	"net/http"

	"github.com/containers/podman/v5/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerQuadletHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/quadlets/json libpod QuadletListLibpod
	// ---
	// tags:
	//   - quadlets
	// summary: List quadlets
	// description: Return a list of all quadlets.
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a map[string][]string).
	//      Supported filters:
	//        - name=<quadlet-name> Filter by quadlet name
	// responses:
	//   200:
	//     $ref: "#/responses/quadletListResponse"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/quadlets/json"), s.APIHandler(libpod.ListQuadlets)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/quadlets/{name}/file libpod QuadletFileLibpod
	// ---
	// tags:
	//   - quadlets
	// summary: Get quadlet file
	// description: Get the contents of a Quadlet, displaying the file including all comments
	// produces:
	// - text/plain
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the quadlet with extension (e.g., "myapp.container")
	// responses:
	//   200:
	//     $ref: "#/responses/quadletFileResponse"
	//   404:
	//     $ref: "#/responses/quadletNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/quadlets/{name}/file"), s.APIHandler(libpod.GetQuadletPrint)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/quadlets libpod QuadletInstallLibpod
	// ---
	// tags:
	//   - quadlets
	// summary: Install quadlet files
	// description: |
	//   Install one or more files for a quadlet application. Each request should contain a single quadlet file
	//   and optionally more files such as containerfile, kube yaml or configuration files. Supports both tar
	//   archives and multipart form data uploads.
	// consumes:
	// - application/x-tar
	// - multipart/form-data
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: replace
	//    type: boolean
	//    default: false
	//    description: Replace the installation files even if the files already exists
	//  - in: query
	//    name: reload-systemd
	//    type: boolean
	//    default: true
	//    description: Reload systemd after installing quadlets
	//  - in: body
	//    name: request
	//    description: |
	//      Quadlet files to install. Can be provided as:
	//      - application/x-tar: A tar archive containing one quadlet file and optionally additional files
	//      - multipart/form-data: One quadlet file as form data and optionally additional files
	//    schema:
	//      type: string
	//      format: binary
	// responses:
	//   200:
	//     description: Quadlet installation report
	//     schema:
	//       type: object
	//       properties:
	//         InstalledQuadlets:
	//           type: object
	//           additionalProperties:
	//             type: string
	//           description: Map of source path to installed path for successfully installed quadlets
	//         QuadletErrors:
	//           type: object
	//           additionalProperties:
	//             type: string
	//           description: Map of source path to error message for failed installations
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/quadlets"), s.APIHandler(libpod.InstallQuadlets)).Methods(http.MethodPost)
	return nil
}
