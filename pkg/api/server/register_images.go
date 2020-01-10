package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	// swagger:operation POST /images/create images createImage
	//
	// Create an image from an image
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: fromImage
	//    type: string
	//    description: needs description
	//  - in: query
	//    name: tag
	//    type: string
	//    description: needs description
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "TBD"
	//   '404':
	//     description: repo or image does not exist
	//     schema:
	//       $ref: "#/responses/generalError"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      $ref: '#/responses/GenericError'
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromImage)).Methods(http.MethodPost).Queries("fromImage", "{fromImage}")
	// swagger:operation POST /images/create images createImage
	//
	// Create an image from Source
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: fromSrc
	//    type: string
	//    description: needs description
	//  - in: query
	//    name: changes
	//    type: TBD
	//    description: needs description
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "TBD"
	//   '404':
	//     description: repo or image does not exist
	//     schema:
	//       $ref: "#/responses/generalError"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      $ref: '#/responses/GenericError'
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromSrc)).Methods(http.MethodPost).Queries("fromSrc", "{fromSrc}")
	// swagger:operation GET /images/json images listImages
	//
	// List Images
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//      schema:
	//        type: array
	//        items:
	//          schema:
	//           $ref: "#/responses/DockerImageSummary"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/json"), APIHandler(s.Context, generic.GetImages)).Methods(http.MethodGet)
	// swagger:operation POST /images/load images loadImage
	//
	// Import image
	//
	// ---
	// parameters:
	//  - in: query
	//    name: quiet
	//    type: bool
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: TBD
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	// swagger:operation POST /images/prune images pruneImages
	//
	// Prune unused images
	//
	// ---
	// parameters:
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsImageDeleteResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/prune"), APIHandler(s.Context, generic.PruneImages)).Methods(http.MethodPost)
	// swagger:operation GET /images/search images searchImages
	//
	// Search images
	//
	// ---
	// parameters:
	//  - in: query
	//    name: term
	//    type: string
	//    description: term to search
	//  - in: query
	//    name: limit
	//    type: int
	//    description: maximum number of results
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: TBD
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsSearchResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/search"), APIHandler(s.Context, handlers.SearchImages)).Methods(http.MethodGet)
	// swagger:operation DELETE /images/{nameOrID} images removeImage
	//
	// Remove Image
	//
	// ---
	// parameters:
	//  - in: query
	//    name: force
	//    type: bool
	//    description: remove the image even if used by containers or has other tags
	//  - in: query
	//    name: noprune
	//    type: bool
	//    description: not supported
	// produces:
	//  - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsImageDeleteResponse"
	//   '404':
	//       $ref: '#/responses/BadParamError'
	//   '409':
	//       $ref: '#/responses/ConflictError'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	// swagger:operation GET /images/{nameOrID}/get images exportImage
	//
	// Export an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	//  - application/json
	// responses:
	//   '200':
	//     schema:
	//       $ref: "TBD"
	//     description: TBD
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/{name:..*}/get"), APIHandler(s.Context, generic.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /images/{nameOrID}/history images imageHistory
	//
	// History of an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     $ref: "#/responses/DocsHistory"
	//   '404':
	//     $ref: "#/responses/NoSuchImage"
	//   '500':
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/images/{name:..*}/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods(http.MethodGet)
	// swagger:operation GET /images/{nameOrID}/json images inspectImage
	//
	// Inspect an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsImageInspect"
	//   '404':
	//       $ref: "#/responses/NoSuchImage"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/images/{name:..*}/json"), APIHandler(s.Context, generic.GetImage))
	// swagger:operation POST /images/{nameOrID}/tag images tagImage
	//
	// Tag an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: repo
	//    type: string
	//    description: the repository to tag in
	//  - in: query
	//    name: tag
	//    type: string
	//    description: the name of the new tag
	// produces:
	// - application/json
	// responses:
	//   201:
	//     description: no error
	//   400:
	//     $ref: '#/responses/BadParamError'
	//   404:
	//     $ref: '#/responses/NoSuchImage'
	//   409:
	//     $ref: '#/responses/ConflictError'
	//   500:
	//     $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods(http.MethodPost)
	// swagger:operation POST /commit/ commit commitContainer
	//
	// Create a new image from a container
	//
	// ---
	// parameters:
	//  - in: query
	//    name: container
	//    type: string
	//    description: the name or ID of a container
	//  - in: query
	//    name: repo
	//    type: string
	//    description: the repository name for the created image
	//  - in: query
	//    name: tag
	//    type: string
	//    description: tag name for the created image
	//  - in: query
	//    name: comment
	//    type: string
	//    description: commit message
	//  - in: query
	//    name: author
	//    type: string
	//    description: author of the image
	//  - in: query
	//    name: pause
	//    type: bool
	//    description: pause the container before committing it
	//  - in: query
	//    name: changes
	//    type: string
	//    description: instructions to apply while committing in Dockerfile format
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     description: no error
	//   '404':
	//      $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/commit"), APIHandler(s.Context, generic.CommitContainer)).Methods(http.MethodPost)

	/*
		libpod endpoints
	*/

	// swagger:operation POST /libpod/images/{nameOrID}/exists images imageExists
	//
	// Check if image exists in local store
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: fromImage
	//    type: string
	//    description: needs description
	//  - in: query
	//    name: tag
	//    type: string
	//    description: needs description
	// responses:
	//   '204':
	//     description: image exists
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/exists"), APIHandler(s.Context, libpod.ImageExists))
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tree"), APIHandler(s.Context, libpod.ImageTree))
	// swagger:operation GET /libpod/images/{nameOrID}/history images imageHistory
	//
	// History of an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/HistoryResponse"
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/json images listImages
	//
	// List Images
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DockerImageSummary"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/json"), APIHandler(s.Context, libpod.GetImages)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/images/load images loadImage
	//
	// Import image
	//
	// ---
	// parameters:
	//  - in: query
	//    name: quiet
	//    type: bool
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: TBD
	//   '500':
	//     $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/images/prune images pruneImages
	//
	// Prune unused images
	//
	// ---
	// parameters:
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: image filters
	//  - in: query
	//    name: all
	//    type: bool
	//    description: prune all images
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsImageDeleteResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/prune"), APIHandler(s.Context, libpod.PruneImages)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/images/search images searchImages
	//
	// Search images
	//
	// ---
	// parameters:
	//  - in: query
	//    name: term
	//    type: string
	//    description: term to search
	//  - in: query
	//    name: limit
	//    type: int
	//    description: maximum number of results
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: TBD
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsSearchResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/search"), APIHandler(s.Context, handlers.SearchImages)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/images/{nameOrID} images removeImage
	//
	// Remove Image
	//
	// ---
	// parameters:
	//  - in: query
	//    name: force
	//    type: bool
	//    description: remove the image even if used by containers or has other tags
	//  - in: query
	//    name: noprune
	//    type: bool
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsIageDeleteResponse"
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '409':
	//       $ref: '#/responses/ConflictError'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/images/{nameOrID}/get images exportImage
	//
	// Export an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: format
	//    type: string
	//    description: format for exported image
	//  - in: query
	//    name: compress
	//    type: bool
	//    description: use compression on image
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: TBD
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/get"), APIHandler(s.Context, libpod.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{nameOrID}/json images inspectImage
	//
	// Inspect an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsLibpodInspectImageResponse"
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/json"), APIHandler(s.Context, libpod.GetImage))
	// swagger:operation POST /libpod/images/{nameOrID}/tag images tagImage
	//
	// Tag an image
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: repo
	//    type: string
	//    description: the repository to tag in
	//  - in: query
	//    name: tag
	//    type: string
	//    description: the name of the new tag
	// produces:
	// - application/json
	// responses:
	//   '201':
	//     description: no error
	//   '400':
	//       $ref: '#/responses/BadParamError'
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '409':
	//       $ref: '#/responses/ConflictError'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods(http.MethodPost)

	r.Handle(VersionedPath("/build"), APIHandler(s.Context, handlers.BuildImage)).Methods(http.MethodPost)
	return nil
}
