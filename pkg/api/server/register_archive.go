package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerAchiveHandlers(r *mux.Router) error {
	// swagger:operation POST /containers/{name}/archive compat putArchive
	// ---
	//  summary: Put files into a container
	//  description: Put a tar archive of files into a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//   - in: query
	//     name: noOverwriteDirNonDir
	//     type: string
	//     description: if unpacking the given content would cause an existing directory to be replaced with a non-directory and vice versa (1 or true)
	//   - in: query
	//     name: copyUIDGID
	//     type: string
	//     description: copy UID/GID maps to the dest file or di (1 or true)
	//   - in: body
	//     name: request
	//     description: tarfile of files to copy into the container
	//     schema:
	//       type: string
	//  responses:
	//    200:
	//      description: no error
	//    400:
	//      $ref: "#/responses/BadParamError"
	//    403:
	//      description: the container rootfs is read-only
	//    404:
	//      $ref: "#/responses/NoSuchContainer"
	//    500:
	//      $ref: "#/responses/InternalError"

	// swagger:operation GET /containers/{name}/archive compat getArchive
	// ---
	//  summary: Get files from a container
	//  description: Get a tar archive of files from a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//  responses:
	//    200:
	//      description: no error
	//      schema:
	//       type: string
	//       format: binary
	//    400:
	//      $ref: "#/responses/BadParamError"
	//    404:
	//      $ref: "#/responses/NoSuchContainer"
	//    500:
	//      $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/archive"), s.APIHandler(compat.Archive)).Methods(http.MethodGet, http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/archive", s.APIHandler(compat.Archive)).Methods(http.MethodGet, http.MethodPost)

	/*
		Libpod
	*/

	// swagger:operation POST /libpod/containers/{name}/copy libpod libpodPutArchive
	// ---
	//  summary: Copy files into a container
	//  description: Copy a tar archive of files into a container
	//  tags:
	//   - containers
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//   - in: query
	//     name: pause
	//     type: boolean
	//     description: pause the container while copying (defaults to true)
	//     default: true
	//   - in: body
	//     name: request
	//     description: tarfile of files to copy into the container
	//     schema:
	//       type: string
	//  responses:
	//    200:
	//      description: no error
	//    400:
	//      $ref: "#/responses/BadParamError"
	//    403:
	//      description: the container rootfs is read-only
	//    404:
	//      $ref: "#/responses/NoSuchContainer"
	//    500:
	//      $ref: "#/responses/InternalError"

	// swagger:operation GET /libpod/containers/{name}/copy libpod libpodGetArchive
	// ---
	//  summary: Copy files from a container
	//  description: Copy a tar archive of files from a container
	//  tags:
	//   - containers (compat)
	//  produces:
	//  - application/json
	//  parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: container name or id
	//     required: true
	//   - in: query
	//     name: path
	//     type: string
	//     description: Path to a directory in the container to extract
	//     required: true
	//  responses:
	//    200:
	//      description: no error
	//      schema:
	//       type: string
	//       format: binary
	//    400:
	//      $ref: "#/responses/BadParamError"
	//    404:
	//      $ref: "#/responses/NoSuchContainer"
	//    500:
	//      $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/copy"), s.APIHandler(libpod.Archive)).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/archive"), s.APIHandler(libpod.Archive)).Methods(http.MethodGet, http.MethodPost)

	return nil
}
