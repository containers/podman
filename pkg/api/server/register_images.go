package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	// swagger:operation POST /images/create compat createImageFromImage
	//
	// ---
	// tags:
	//  - images (compat)
	// summary: Create an image from an image
	// description: Create an image by either pulling it from a registry or importing it.
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
	//       $ref: "to be determined"
	//   '404':
	//     description: repo or image does not exist
	//     schema:
	//       $ref: "#/responses/InternalError"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      $ref: '#/responses/GenericError'
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromImage)).Methods(http.MethodPost).Queries("fromImage", "{fromImage}")
	// swagger:operation POST /images/create compat createImageFromSrc
	// ---
	// tags:
	//  - images (compat)
	// summary: Create an image from Source
	// description: Create an image by either pulling it from a registry or importing it.
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: fromSrc
	//    type: string
	//    description: needs description
	//  - in: query
	//    name: changes
	//    type: to be determined
	//    description: needs description
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "to be determined"
	//   '404':
	//     description: repo or image does not exist
	//     schema:
	//       $ref: "#/responses/InternalError"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      $ref: '#/responses/GenericError'
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromSrc)).Methods(http.MethodPost).Queries("fromSrc", "{fromSrc}")
	// swagger:operation GET /images/json compat getImages
	// ---
	// tags:
	//  - images (compat)
	// summary: List Images
	// description: Returns a list of images on the server. Note that it uses a different, smaller representation of an image than inspecting a single image.
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
	// swagger:operation POST /images/load compat loadImage
	//
	// ---
	// tags:
	//  - images (compat)
	// summary: Import image
	// description: Load a set of images and tags into a repository.
	// parameters:
	//  - in: query
	//    name: quiet
	//    type: bool
	//    description: not supported
	//  - in: body
	//    description: tarball of container image
	//    type: string
	//    format: binary
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: no error
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	// swagger:operation POST /images/prune compat pruneImages
	// ---
	// tags:
	//  - images (compat)
	// summary: Prune unused images
	// description: Remove images from local storage that are not being used by a container
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      filters to apply to image pruning, encoded as JSON (map[string][]string). Available filters:
	//        - `dangling=<boolean>` When set to `true` (or `1`), prune only
	//           unused *and* untagged images. When set to `false`
	//           (or `0`), all unused images are pruned.
	//        - `until=<string>` Prune images created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune images with (or without, in case `label!=...` is used) the specified labels.
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
	// swagger:operation GET /images/search compat searchImages
	// ---
	// tags:
	//  - images (compat)
	// summary: Search images
	// description: Search registries for an image
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
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `is-automated=(true|false)`
	//        - `is-official=(true|false)`
	//        - `stars=<number>` Matches images that has at least 'number' stars.
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsSearchResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/search"), APIHandler(s.Context, handlers.SearchImages)).Methods(http.MethodGet)
	// swagger:operation DELETE /images/{nameOrID} compat removeImage
	// ---
	// tags:
	//  - images (compat)
	// summary: Remove Image
	// description: Delete an image from local storage
	// parameters:
	//  - in: query
	//    name: force
	//    type: bool
	//    description: remove the image even if used by containers or has other tags
	//  - in: query
	//    name: noprune
	//    type: bool
	//    description: not supported. will be logged as an invalid parameter if enabled
	// produces:
	//  - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsImageDeleteResponse"
	//   '400':
	//       $ref: '#/responses/BadParamError'
	//   '409':
	//       $ref: '#/responses/ConflictError'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	// swagger:operation GET /images/{nameOrID}/get compat exportImage
	// ---
	// tags:
	//  - images (compat)
	// summary: Export an image
	// description: Export an image in tarball format
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	//  - application/json
	// responses:
	//   '200':
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/images/{name:..*}/get"), APIHandler(s.Context, generic.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /images/{nameOrID}/history compat historyImage
	// ---
	// tags:
	//  - images (compat)
	// summary: History of an image
	// description: Return parent layers of an image.
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
	// swagger:operation GET /images/{nameOrID}/json compat getImage
	// ---
	// tags:
	//  - images (compat)
	// summary: Inspect an image
	// description: Return low-level information about an image.
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
	r.Handle(VersionedPath("/images/{name:..*}/json"), APIHandler(s.Context, generic.GetImage)).Methods(http.MethodGet)
	// swagger:operation POST /images/{nameOrID}/tag compat tagImage
	// ---
	// tags:
	//  - images (compat)
	// summary: Tag an image
	// description: Tag an image so that it becomes part of a repository.
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
	// swagger:operation POST /commit compat commitContainer
	// ---
	// tags:
	//  - commit (compat)
	// summary: Create a new image from a container
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

	// swagger:operation GET /libpod/images/{nameOrID}/exists libpod libpodImageExists
	// ---
	// tags:
	//  - images
	// summary: Image exists
	// description: Check if image exists in local store
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: image exists
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/exists"), APIHandler(s.Context, libpod.ImageExists)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{nameOrID}/tree libpod libpodImageTree
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tree"), APIHandler(s.Context, libpod.ImageTree)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/history libpod historyImage
	// ---
	// tags:
	//  - images
	// summary: History of an image
	// description: Return parent layers of an image.
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
	// swagger:operation GET /libpod/images/json libpod libpodGetImages
	// ---
	// tags:
	//  - images
	// summary: List Images
	// description: Returns a list of images on the server
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DockerImageSummary"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/json"), APIHandler(s.Context, libpod.GetImages)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/images/load libpod loadImage
	// ---
	// tags:
	//  - images
	// summary: Import image
	// description: Load a set of images and tags into a repository.
	// parameters:
	//  - in: query
	//    name: quiet
	//    type: bool
	//    description: not supported
	//  - in: body
	//    description: tarball of container image
	//    type: string
	//    format: binary
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: no error
	//   '500':
	//     $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/images/prune libpod libpodPruneImages
	// ---
	// tags:
	//  - images
	// summary: Prune unused images
	// description: Remove images that are not being used by a container
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      filters to apply to image pruning, encoded as JSON (map[string][]string). Available filters:
	//        - `dangling=<boolean>` When set to `true` (or `1`), prune only
	//           unused *and* untagged images. When set to `false`
	//           (or `0`), all unused images are pruned.
	//        - `until=<string>` Prune images created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune images with (or without, in case `label!=...` is used) the specified labels.
	//  - in: query
	//    name: all
	//    type: bool
	//    description: prune all images
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     items:
	//       $ref: "#/responses/DocsImageDeleteResponse"
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/prune"), APIHandler(s.Context, libpod.PruneImages)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/images/search libpod searchImages
	// ---
	// tags:
	//  - images
	// summary: Search images
	// description: Search registries for images
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
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `is-automated=(true|false)`
	//        - `is-official=(true|false)`
	//        - `stars=<number>` Matches images that has at least 'number' stars.
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
	// swagger:operation DELETE /libpod/images/{nameOrID} libpod removeImage
	// ---
	// tags:
	//  - images
	// summary: Remove Image
	// description: Delete an image from local store
	// parameters:
	//  - in: query
	//    name: force
	//    type: bool
	//    description: remove the image even if used by containers or has other tags
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       $ref: "#/responses/DocsIageDeleteResponse"
	//   '400':
	//       $ref: "#/responses/BadParamError"
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '409':
	//       $ref: '#/responses/ConflictError'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/images/{nameOrID}/get libpod libpodExportImage
	// ---
	// tags:
	//  - images
	// summary: Export an image
	// description: Export an image as a tarball
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
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/get"), APIHandler(s.Context, libpod.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{nameOrID}/json libpod libpodGetImage
	// ---
	// tags:
	//  - images
	// summary: Inspect an image
	// description: Obtain low-level information about an image
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       $ref: "#/responses/DocsLibpodInspectImageResponse"
	//   '404':
	//       $ref: '#/responses/NoSuchImage'
	//   '500':
	//      $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/images/{name:..*}/json"), APIHandler(s.Context, libpod.GetImage)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/images/{nameOrID}/tag libpod tagImage
	// ---
	// tags:
	//  - images
	// summary: Tag an image
	// description: Tag an image so that it becomes part of a repository.
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

	// swagger:operation POST /build compat buildImage
	r.Handle(VersionedPath("/build"), APIHandler(s.Context, handlers.BuildImage)).Methods(http.MethodPost)
	return nil
}
