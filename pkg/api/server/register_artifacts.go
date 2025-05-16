//go:build !remote

package server

import (
	"net/http"

	"github.com/containers/podman/v5/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerArtifactHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/artifacts/{name}/json libpod ArtifactInspectLibpod
	// ---
	// summary: Inspect an artifact
	// description: Obtain low-level information about an artifact
	// tags:
	//   - artifacts
	// produces:
	//   - application/json
	// parameters:
	//   - in: path
	//     name: name
	//     type: string
	//     description: The name or ID of the artifact
	//     required: true
	// responses:
	//   200:
	//     $ref: "#/responses/inspectArtifactResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/artifacts/{name:.*}/json"), s.APIHandler(libpod.InspectArtifact)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/artifacts/json libpod ArtifactListLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: List artifacts
	// description: Returns a list of artifacts on the server.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/artifactListResponse"
	//   500:
	//     $ref: '#/responses/internalError'
	r.HandleFunc(VersionedPath("/libpod/artifacts/json"), s.APIHandler(libpod.ListArtifact)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/artifacts/pull libpod ArtifactPullLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Pull an OCI artifact
	// description: Pulls an artifact from a registry and stores it locally.
	// parameters:
	//   - in: query
	//     name: name
	//     description: "Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)"
	//     type: string
	//   - in: query
	//     name: retry
	//     description: "Number of times to retry in case of failure when performing pull"
	//     type: integer
	//     default: 3
	//   - in: query
	//     name: retryDelay
	//     description: "Delay between retries in case of pull failures (e.g., 10s)"
	//     type: string
	//     default: 1s
	//   - in: query
	//     name: tlsVerify
	//     description: Require TLS verification.
	//     type: boolean
	//     default: true
	//   - in: header
	//     name: X-Registry-Auth
	//     description: "base-64 encoded auth config. Must include the following four values: username, password, email and server address OR simply just an identity token."
	//     type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/artifactPullResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/artifacts/pull"), s.APIHandler(libpod.PullArtifact)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/artifacts/{name} libpod ArtifactDeleteLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Remove Artifact
	// description: Delete an Artifact from local storage
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: name or ID of artifact to delete
	// produces:
	//  - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/artifactRemoveResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}"), s.APIHandler(libpod.RemoveArtifact)).Methods(http.MethodDelete)
	// swagger:operation POST /libpod/artifacts/add libpod ArtifactAddLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Add an OCI artifact to the local store
	// description: Add an OCI artifact to the local store from the local filesystem
	// parameters:
	//   - in: query
	//     name: name
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     type: string
	//     required: true
	//   - in: query
	//     name: fileName
	//     description: File to be added to the artifact
	//     type: string
	//     required: true
	//   - in: query
	//     name: annotations
	//     description: JSON encoded value of annotations (a map[string]string)
	//     type: string
	//   - in: query
	//     name: artifactMIMEType
	//     description: Use type to describe an artifact
	//     type: string
	//   - in: query
	//     name: append
	//     description: Append files to an existing artifact
	//     type: boolean
	//     default: false
	//   - in: body
	//     name: inputStream
	//     description: |
	//       A binary stream of the blob to add
	//     schema:
	//       type: string
	//       format: binary
	// produces:
	//   - application/json
	// consumes:
	//   - application/octet-stream
	// responses:
	//   201:
	//     $ref: "#/responses/artifactAddResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/artifacts/add"), s.APIHandler(libpod.AddArtifact)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/artifacts/{name}/push libpod ArtifactPushLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Push an OCI artifact
	// description: Push an OCI artifact from local storage to an image registry.
	// parameters:
	//   - in: path
	//     name: name
	//     description: "Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)"
	//     type: string
	//     required: true
	//   - in: query
	//     name: retry
	//     description: "Number of times to retry in case of failure when performing pull"
	//     type: integer
	//     default: 3
	//   - in: query
	//     name: retryDelay
	//     description: "Delay between retries in case of pull failures (e.g., 10s)"
	//     type: string
	//     default: 1s
	//   - in: query
	//     name: tlsVerify
	//     description: Require TLS verification.
	//     type: boolean
	//     default: true
	//   - in: header
	//     name: X-Registry-Auth
	//     description: "base-64 encoded auth config. Must include the following four values: username, password, email and server address OR simply just an identity token."
	//     type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/artifactPushResponse'
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/push"), s.APIHandler(libpod.PushArtifact)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/artifacts/{name}/extract libpod ArtifactExtractLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Extract an OCI artifact to a local path
	// description: Extract the blobs of an OCI artifact to a local file or directory
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: The name or digest of artifact
	//  - in: query
	//    name: title
	//    type: string
	//    description: Only extract blob with the given title
	//  - in: query
	//    name: digest
	//    type: string
	//    description: Only extract blob with the given digest
	// produces:
	//   - application/x-tar
	// responses:
	//   200:
	//     description: "Extract successful" # FIX: Should 200 return confirmation text, I think no response is okay?
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/extract"), s.APIHandler(libpod.ExtractArtifact)).Methods(http.MethodGet)
	return nil
}
