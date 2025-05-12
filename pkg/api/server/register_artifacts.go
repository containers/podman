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
	// tags:
	//   - artifacts
	// summary: Inspect an artifact
	// description: Obtain low-level information about an artifact
	// produces:
	//   - application/json
	// parameters:
	//   - name: name
	//     in: path
	//     description: The name or ID of the artifact
	//     required: true
	//     type: string
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
	//   - artifacts
	// summary: List artifacts
	// description: Returns a list of artifacts on the server.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/artifactListResponse"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/artifacts/json"), s.APIHandler(libpod.ListArtifact)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/artifacts/pull libpod ArtifactPullLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Pull an OCI artifact
	// description: Pulls an artifact from a registry and stores it locally.
	// produces:
	// - application/json
	// parameters:
	//   - name: name
	//     in: query
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: retry
	//     in: query
	//     description: Number of times to retry in case of failure when performing pull
	//     type: integer
	//     default: 3
	//   - name: retryDelay
	//     in: query
	//     description: Delay between retries in case of pull failures (e.g., 10s)
	//     type: string
	//     default: 1s
	//   - name: tlsVerify
	//     in: query
	//     description: Require TLS verification.
	//     type: boolean
	//     default: true
	//   - name: X-Registry-Auth
	//     in: header
	//     description: |
	//       base-64 encoded auth config.
	//       Must include the following four values: username, password, email and server address
	//       OR simply just an identity token.
	//     type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactPullResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   401:
	//     $ref: "#/responses/artifactBadAuth"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/pull"), s.APIHandler(libpod.PullArtifact)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/artifacts/{name} libpod ArtifactDeleteLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Remove Artifact
	// description: Delete an Artifact from local storage
	// produces:
	//  - application/json
	// parameters:
	//  - name: name
	//    in: path
	//    description: name or ID of artifact to delete
	//    required: true
	//    type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactRemoveResponse"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}"), s.APIHandler(libpod.RemoveArtifact)).Methods(http.MethodDelete)
	// swagger:operation POST /libpod/artifacts/add libpod ArtifactAddLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Add an OCI artifact to the local store
	// description: Add an OCI artifact to the local store from the local filesystem
	// produces:
	//   - application/json
	// consumes:
	//   - application/octet-stream
	// parameters:
	//   - name: name
	//     in: query
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: fileName
	//     in: query
	//     description: File to be added to the artifact
	//     required: true
	//     type: string
	//   - name: fileMIMEType
	//     in: query
	//     description: Optionally set the type of file
	//     type: string
	//   - name: annotations
	//     in: query
	//     description: Array of annotation strings e.g "test=true"
	//     type: array
	//     items:
	//       type: string
	//   - name: artifactMIMEType
	//     in: query
	//     description: Use type to describe an artifact
	//     type: string
	//   - name: append
	//     in: query
	//     description: Append files to an existing artifact
	//     type: boolean
	//     default: false
	//   - name: inputStream
	//     in: body
	//     description: A binary stream of the blob to add to artifact
	//     schema:
	//       type: string
	//       format: binary
	// responses:
	//   201:
	//     $ref: "#/responses/artifactAddResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/add"), s.APIHandler(libpod.AddArtifact)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/artifacts/{name}/push libpod ArtifactPushLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Push an OCI artifact
	// description: Push an OCI artifact from local storage to an image registry.
	// produces:
	//   - application/json
	// parameters:
	//   - name: name
	//     in: path
	//     description: Mandatory reference to the artifact (e.g., quay.io/image/artifact:tag)
	//     required: true
	//     type: string
	//   - name: retry
	//     in: query
	//     description: Number of times to retry in case of failure when performing pull
	//     type: integer
	//     default: 3
	//   - name: retryDelay
	//     in: query
	//     description: Delay between retries in case of pull failures (e.g., 10s)
	//     type: string
	//     default: 1s
	//   - name: tlsVerify
	//     in: query
	//     description: Require TLS verification.
	//     type: boolean
	//     default: true
	//   - name: X-Registry-Auth
	//     in: header
	//     description: |
	//       base-64 encoded auth config.
	//       Must include the following four values: username, password, email and server address
	//       OR simply just an identity token.
	//     type: string
	// responses:
	//   200:
	//     $ref: "#/responses/artifactPushResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   401:
	//     $ref: "#/responses/artifactBadAuth"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/push"), s.APIHandler(libpod.PushArtifact)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/artifacts/{name}/extract libpod ArtifactExtractLibpod
	// ---
	// tags:
	//  - artifacts
	// summary: Extract an OCI artifact to a local path
	// description: Extract the blobs of an OCI artifact to a local file or directory
	// produces:
	//   - application/x-tar
	// parameters:
	//  - name: name
	//    in: path
	//    description: The name or digest of artifact
	//    required: true
	//    type: string
	//  - name: title
	//    in: query
	//    description: Only extract blob with the given title
	//    type: string
	//  - name: digest
	//    in: query
	//    description: Only extract blob with the given digest
	//    type: string
	// responses:
	//   200:
	//     description: Extract successful
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: "#/responses/artifactNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/artifacts/{name:.*}/extract"), s.APIHandler(libpod.ExtractArtifact)).Methods(http.MethodGet)
	return nil
}
