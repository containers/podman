package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/compat"
	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

// TODO
//
// * /images/create is missing the "message" and "platform" parameters

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	// swagger:operation POST /images/create compat ImageCreate
	// ---
	// tags:
	//  - images (compat)
	// summary: Create an image
	// description: Create an image by either pulling it from a registry or importing it.
	// consumes:
	// - text/plain
	// - application/octet-stream
	// produces:
	// - application/json
	// parameters:
	//  - in: header
	//    name: X-Registry-Auth
	//    type: string
	//    description: A base64-encoded auth configuration.
	//  - in: query
	//    name: fromImage
	//    type: string
	//    description: Name of the image to pull. The name may include a tag or digest. This parameter may only be used when pulling an image. The pull is cancelled if the HTTP connection is closed.
	//  - in: query
	//    name: fromSrc
	//    type: string
	//    description: Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image
	//  - in: query
	//    name: repo
	//    type: string
	//    description: Repository name given to an image when it is imported. The repo may include a tag. This parameter may only be used when importing an image.
	//  - in: query
	//    name: tag
	//    type: string
	//    description: Tag or digest. If empty when pulling an image, this causes all tags for the given image to be pulled.
	//  - in: query
	//    name: message
	//    type: string
	//    description: Set commit message for imported image.
	//  - in: query
	//    name: platform
	//    type: string
	//    description: Platform in the format os[/arch[/variant]]
	//  - in: body
	//    name: inputImage
	//    schema:
	//      type: string
	//      format: binary
	//    description: Image content if fromSrc parameter was used
	// responses:
	//   200:
	//     description: "no error"
	//     schema:
	//       type: "string"
	//       format: "binary"
	//   404:
	//     $ref: "#/responses/imageNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/images/create"), s.APIHandler(compat.CreateImageFromImage)).Methods(http.MethodPost).Queries("fromImage", "{fromImage}")
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/create", s.APIHandler(compat.CreateImageFromImage)).Methods(http.MethodPost).Queries("fromImage", "{fromImage}")
	r.Handle(VersionedPath("/images/create"), s.APIHandler(compat.CreateImageFromSrc)).Methods(http.MethodPost).Queries("fromSrc", "{fromSrc}")
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/create", s.APIHandler(compat.CreateImageFromSrc)).Methods(http.MethodPost).Queries("fromSrc", "{fromSrc}")
	// swagger:operation GET /images/json compat ImageList
	// ---
	// tags:
	//  - images (compat)
	// summary: List Images
	// description: Returns a list of images on the server. Note that it uses a different, smaller representation of an image than inspecting a single image.
	// parameters:
	//   - name: all
	//     in: query
	//     description: "Show all images. Only images from a final layer (no children) are shown by default."
	//     type: boolean
	//     default: false
	//   - name: filters
	//     in: query
	//     description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `before`=(`<image-name>[:<tag>]`,  `<image id>` or `<image@digest>`)
	//        - `dangling=true`
	//        - `label=key` or `label="key=value"` of an image label
	//        - `reference`=(`<image-name>[:<tag>]`)
	//        - `since`=(`<image-name>[:<tag>]`,  `<image id>` or `<image@digest>`)
	//     type: string
	//   - name: digests
	//     in: query
	//     description: Not supported
	//     type: boolean
	//     default: false
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imageList"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/json"), s.APIHandler(compat.GetImages)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/json", s.APIHandler(compat.GetImages)).Methods(http.MethodGet)
	// swagger:operation POST /images/load compat ImageLoad
	// ---
	// tags:
	//  - images (compat)
	// summary: Import image
	// description: Load a set of images and tags into a repository.
	// parameters:
	//  - in: query
	//    name: quiet
	//    type: boolean
	//    description: not supported
	//  - in: body
	//    name: request
	//    description: tarball of container image
	//    schema:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/load"), s.APIHandler(compat.LoadImages)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/load", s.APIHandler(compat.LoadImages)).Methods(http.MethodPost)
	// swagger:operation POST /images/prune compat ImagePrune
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
	//   200:
	//     $ref: "#/responses/imageDeleteResponse"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/prune"), s.APIHandler(compat.PruneImages)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/prune", s.APIHandler(compat.PruneImages)).Methods(http.MethodPost)
	// swagger:operation GET /images/search compat ImageSearch
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
	//    type: integer
	//    default: 25
	//    description: maximum number of results
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `is-automated=(true|false)`
	//        - `is-official=(true|false)`
	//        - `stars=<number>` Matches images that has at least 'number' stars.
	//  - in: query
	//    name: tlsVerify
	//    type: boolean
	//    default: true
	//    description: Require HTTPS and verify signatures when contacting registries.
	//  - in: query
	//    name: listTags
	//    type: boolean
	//    description: list the available tags in the repository
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/registrySearchResponse"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/search"), s.APIHandler(compat.SearchImages)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/search", s.APIHandler(compat.SearchImages)).Methods(http.MethodGet)
	// swagger:operation DELETE /images/{name} compat ImageDelete
	// ---
	// tags:
	//  - images (compat)
	// summary: Remove Image
	// description: Delete an image from local storage
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: name or ID of image to delete
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: remove the image even if used by containers or has other tags
	//  - in: query
	//    name: noprune
	//    type: boolean
	//    description: not supported. will be logged as an invalid parameter if enabled
	// produces:
	//  - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imageDeleteResponse"
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   409:
	//     $ref: '#/responses/conflictError'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/{name:.*}"), s.APIHandler(compat.RemoveImage)).Methods(http.MethodDelete)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}", s.APIHandler(compat.RemoveImage)).Methods(http.MethodDelete)
	// swagger:operation POST /images/{name}/push compat ImagePush
	// ---
	// tags:
	//  - images (compat)
	// summary: Push Image
	// description: Push an image to a container registry
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Name of image to push.
	//  - in: query
	//    name: tag
	//    type: string
	//    description: The tag to associate with the image on the registry.
	//  - in: query
	//    name: all
	//    type: boolean
	//    description: All indicates whether to push all images related to the image list
	//  - in: query
	//    name: compress
	//    type: boolean
	//    description: use compression on image
	//  - in: query
	//    name: destination
	//    type: string
	//    description: destination name for the image being pushed
	//  - in: header
	//    name: X-Registry-Auth
	//    type: string
	//    description: A base64-encoded auth configuration.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/{name:.*}/push"), s.APIHandler(compat.PushImage)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}/push", s.APIHandler(compat.PushImage)).Methods(http.MethodPost)
	// swagger:operation GET /images/{name}/get compat ImageGet
	// ---
	// tags:
	//  - images (compat)
	// summary: Export an image
	// description: Export an image in tarball format
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	//  - application/x-tar
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/{name:.*}/get"), s.APIHandler(compat.ExportImage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}/get", s.APIHandler(compat.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /images/get compat ImageGetAll
	// ---
	// tags:
	//  - images (compat)
	// summary: Export several images
	// description: Get a tarball containing all images and metadata for several image repositories
	// parameters:
	//  - in:  query
	//    name:  names
	//    type: string
	//    required: true
	//    description: one or more image names or IDs comma separated
	// produces:
	//  - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/get"), s.APIHandler(compat.ExportImages)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/get", s.APIHandler(compat.ExportImages)).Methods(http.MethodGet)
	// swagger:operation GET /images/{name}/history compat ImageHistory
	// ---
	// tags:
	//  - images (compat)
	// summary: History of an image
	// description: Return parent layers of an image.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/history"
	//   404:
	//     $ref: "#/responses/imageNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/images/{name:.*}/history"), s.APIHandler(compat.HistoryImage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}/history", s.APIHandler(compat.HistoryImage)).Methods(http.MethodGet)
	// swagger:operation GET /images/{name}/json compat ImageInspect
	// ---
	// tags:
	//  - images (compat)
	// summary: Inspect an image
	// description: Return low-level information about an image.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imageInspect"
	//   404:
	//     $ref: "#/responses/imageNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/images/{name:.*}/json"), s.APIHandler(compat.GetImage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}/json", s.APIHandler(compat.GetImage)).Methods(http.MethodGet)
	// swagger:operation POST /images/{name}/tag compat ImageTag
	// ---
	// tags:
	//  - images (compat)
	// summary: Tag an image
	// description: Tag an image so that it becomes part of a repository.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
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
	//     $ref: '#/responses/badParamError'
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   409:
	//     $ref: '#/responses/conflictError'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/images/{name:.*}/tag"), s.APIHandler(compat.TagImage)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/images/{name:.*}/tag", s.APIHandler(compat.TagImage)).Methods(http.MethodPost)
	// swagger:operation POST /commit compat ImageCommit
	// ---
	// tags:
	//  - containers (compat)
	// summary: New Image
	// description: Create a new image from a container
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
	//    type: boolean
	//    description: pause the container before committing it
	//  - in: query
	//    name: changes
	//    type: string
	//    description: instructions to apply while committing in Dockerfile format
	//  - in: query
	//    name: squash
	//    type: boolean
	//    description: squash newly built layers into a single new layer
	// produces:
	// - application/json
	// responses:
	//   201:
	//     description: no error
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/commit"), s.APIHandler(compat.CommitContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/commit", s.APIHandler(compat.CommitContainer)).Methods(http.MethodPost)

	// swagger:operation POST /build compat ImageBuild
	// ---
	// tags:
	//  - images (compat)
	// summary: Create image
	// description: Build an image from the given Dockerfile(s)
	// parameters:
	//  - in: header
	//    name: Content-Type
	//    type: string
	//    default: application/x-tar
	//    enum: ["application/x-tar"]
	//  - in: header
	//    name: X-Registry-Config
	//    type: string
	//  - in: query
	//    name: dockerfile
	//    type: string
	//    default: Dockerfile
	//    description: |
	//      Path within the build context to the `Dockerfile`.
	//      This is ignored if remote is specified and points to an external `Dockerfile`.
	//  - in: query
	//    name: t
	//    type: string
	//    default: latest
	//    description: A name and optional tag to apply to the image in the `name:tag` format. If you omit the tag the default latest value is assumed. You can provide several t parameters.
	//  - in: query
	//    name: extrahosts
	//    type: string
	//    default:
	//    description: |
	//      TBD Extra hosts to add to /etc/hosts
	//      (As of version 1.xx)
	//  - in: query
	//    name: remote
	//    type: string
	//    default:
	//    description: |
	//      A Git repository URI or HTTP/HTTPS context URI.
	//      If the URI points to a single text file, the file’s contents are placed
	//      into a file called Dockerfile and the image is built from that file. If
	//      the URI points to a tarball, the file is downloaded by the daemon and the
	//      contents therein used as the context for the build. If the URI points to a
	//      tarball and the dockerfile parameter is also specified, there must be a file
	//      with the corresponding path inside the tarball.
	//      (As of version 1.xx)
	//  - in: query
	//    name: q
	//    type: boolean
	//    default: false
	//    description: |
	//      Suppress verbose build output
	//  - in: query
	//    name: nocache
	//    type: boolean
	//    default: false
	//    description: |
	//      Do not use the cache when building the image
	//      (As of version 1.xx)
	//  - in: query
	//    name: cachefrom
	//    type: string
	//    default:
	//    description: |
	//      JSON array of images used to build cache resolution
	//      (As of version 1.xx)
	//  - in: query
	//    name: pull
	//    type: boolean
	//    default: false
	//    description: |
	//      Attempt to pull the image even if an older image exists locally
	//      (As of version 1.xx)
	//  - in: query
	//    name: rm
	//    type: boolean
	//    default: true
	//    description: |
	//      Remove intermediate containers after a successful build
	//      (As of version 1.xx)
	//  - in: query
	//    name: forcerm
	//    type: boolean
	//    default: false
	//    description: |
	//      Always remove intermediate containers, even upon failure
	//      (As of version 1.xx)
	//  - in: query
	//    name: memory
	//    type: integer
	//    description: |
	//      Memory is the upper limit (in bytes) on how much memory running containers can use
	//      (As of version 1.xx)
	//  - in: query
	//    name: memswap
	//    type: integer
	//    description: |
	//      MemorySwap limits the amount of memory and swap together
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpushares
	//    type: integer
	//    description: |
	//      CPUShares (relative weight
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpusetcpus
	//    type: string
	//    description: |
	//      CPUSetCPUs in which to allow execution (0-3, 0,1)
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpuperiod
	//    type: integer
	//    description: |
	//      CPUPeriod limits the CPU CFS (Completely Fair Scheduler) period
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpuquota
	//    type: integer
	//    description: |
	//      CPUQuota limits the CPU CFS (Completely Fair Scheduler) quota
	//      (As of version 1.xx)
	//  - in: query
	//    name: buildargs
	//    type: string
	//    default:
	//    description: |
	//      JSON map of string pairs denoting build-time variables.
	//      For example, the build argument `Foo` with the value of `bar` would be encoded in JSON as `["Foo":"bar"]`.
	//
	//      For example, buildargs={"Foo":"bar"}.
	//
	//      Note(s):
	//      * This should not be used to pass secrets.
	//      * The value of buildargs should be URI component encoded before being passed to the API.
	//
	//      (As of version 1.xx)
	//  - in: query
	//    name: shmsize
	//    type: integer
	//    default: 67108864
	//    description: |
	//      ShmSize is the "size" value to use when mounting an shmfs on the container's /dev/shm directory.
	//      Default is 64MB
	//      (As of version 1.xx)
	//  - in: query
	//    name: squash
	//    type: boolean
	//    default: false
	//    description: |
	//      Silently ignored.
	//      Squash the resulting images layers into a single layer
	//      (As of version 1.xx)
	//  - in: query
	//    name: labels
	//    type: string
	//    default:
	//    description: |
	//      JSON map of key, value pairs to set as labels on the new image
	//      (As of version 1.xx)
	//  - in: query
	//    name: networkmode
	//    type: string
	//    default: bridge
	//    description: |
	//      Sets the networking mode for the run commands during build.
	//      Supported standard values are:
	//        * `bridge` limited to containers within a single host, port mapping required for external access
	//        * `host` no isolation between host and containers on this network
	//        * `none` disable all networking for this container
	//        * container:<nameOrID> share networking with given container
	//        ---All other values are assumed to be a custom network's name
	//      (As of version 1.xx)
	//  - in: query
	//    name: platform
	//    type: string
	//    default:
	//    description: |
	//      Platform format os[/arch[/variant]]
	//      (As of version 1.xx)
	//  - in: query
	//    name: target
	//    type: string
	//    default:
	//    description: |
	//      Target build stage
	//      (As of version 1.xx)
	//  - in: query
	//    name: outputs
	//    type: string
	//    default:
	//    description: |
	//      output configuration TBD
	//      (As of version 1.xx)
	//  - in: body
	//    name: inputStream
	//    description: |
	//      A tar archive compressed with one of the following algorithms:
	//      identity (no compression), gzip, bzip2, xz.
	//    schema:
	//      type: string
	//      format: binary
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: OK (As of version 1.xx)
	//     schema:
	//       type: object
	//       required:
	//         - stream
	//       properties:
	//         stream:
	//           type: string
	//           description: output from build process
	//           example: |
	//             (build details...)
	//             Successfully built 8ba084515c724cbf90d447a63600c0a6
	//             Successfully tagged your_image:latest
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/build"), s.StreamBufferedAPIHandler(compat.BuildImage)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/build", s.StreamBufferedAPIHandler(compat.BuildImage)).Methods(http.MethodPost)
	/*
		libpod endpoints
	*/

	// swagger:operation POST /libpod/images/{name}/push libpod ImagePushLibpod
	// ---
	// tags:
	//  - images
	// summary: Push Image
	// description: Push an image to a container registry
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Name of image to push.
	//  - in: query
	//    name: destination
	//    type: string
	//    description: Allows for pushing the image to a different destination than the image refers to.
	//  - in: query
	//    name: tlsVerify
	//    description: Require TLS verification.
	//    type: boolean
	//    default: true
	//  - in: query
	//    name: quiet
	//    description: "silences extra stream data on push"
	//    type: boolean
	//    default: true
	//  - in: header
	//    name: X-Registry-Auth
	//    type: string
	//    description: A base64-encoded auth configuration.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/push"), s.APIHandler(libpod.PushImage)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/images/{name}/exists libpod ImageExistsLibpod
	// ---
	// tags:
	//  - images
	// summary: Image exists
	// description: Check if image exists in local store
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: image exists
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/exists"), s.APIHandler(libpod.ImageExists)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{name}/tree libpod ImageTreeLibpod
	// ---
	// tags:
	//  - images
	// summary: Image tree
	// description: Retrieve the image tree for the provided image name or ID
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: whatrequires
	//    type: boolean
	//    description: show all child images and layers of the specified image
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/treeResponse"
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/tree"), s.APIHandler(libpod.ImageTree)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{name}/history libpod ImageHistoryLibpod
	// ---
	// tags:
	//  - images
	// summary: History of an image
	// description: Return parent layers of an image.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/history"
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/history"), s.APIHandler(compat.HistoryImage)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/json libpod ImageListLibpod
	// ---
	// tags:
	//  - images
	// summary: List Images
	// description: Returns a list of images on the server
	// parameters:
	//   - name: all
	//     in: query
	//     description: Show all images. Only images from a final layer (no children) are shown by default.
	//     type: boolean
	//     default: false
	//   - name: filters
	//     in: query
	//     description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `before`=(`<image-name>[:<tag>]`,  `<image id>` or `<image@digest>`)
	//        - `dangling=true`
	//        - `label=key` or `label="key=value"` of an image label
	//        - `reference`=(`<image-name>[:<tag>]`)
	//        - `id`=(`<image-id>`)
	//        - `since`=(`<image-name>[:<tag>]`,  `<image id>` or `<image@digest>`)
	//     type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imageListLibpod"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/json"), s.APIHandler(compat.GetImages)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/images/load libpod ImageLoadLibpod
	// ---
	// tags:
	//  - images
	// summary: Load image
	// description: Load an image (oci-archive or docker-archive) stream.
	// parameters:
	//   - in: body
	//     name: upload
	//     required: true
	//     description: tarball of container image
	//     schema:
	//       type: string
	// consumes:
	// - application/x-tar
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imagesLoadResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/load"), s.APIHandler(libpod.ImagesLoad)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/images/import libpod ImageImportLibpod
	// ---
	// tags:
	//  - images
	// summary: Import image
	// description: Import a previously exported tarball as an image.
	// parameters:
	//   - in: header
	//     name: Content-Type
	//     type: string
	//     default: application/x-tar
	//     enum: ["application/x-tar"]
	//   - in: query
	//     name: changes
	//     description: "Apply the following possible instructions to the created image: CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR.  JSON encoded string"
	//     type: array
	//     items:
	//       type: string
	//   - in: query
	//     name: message
	//     description: Set commit message for imported image
	//     type: string
	//   - in: query
	//     name: reference
	//     description: "Optional Name[:TAG] for the image"
	//     type: string
	//   - in: query
	//     name: url
	//     description: Load image from the specified URL
	//     type: string
	//   - in: body
	//     name: upload
	//     required: true
	//     description: tarball for imported image
	//     schema:
	//       type: string
	//       format: binary
	// produces:
	// - application/json
	// consumes:
	// - application/x-tar
	// responses:
	//   200:
	//     $ref: "#/responses/imagesImportResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/import"), s.APIHandler(libpod.ImagesImport)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/images/remove libpod ImageDeleteAllLibpod
	// ---
	// tags:
	//  - images
	// summary: Remove one or more images from the storage.
	// description: Remove one or more images from the storage.
	// parameters:
	//   - in: query
	//     name: images
	//     description: Images IDs or names to remove.
	//     type: array
	//     items:
	//        type: string
	//   - in: query
	//     name: all
	//     description: Remove all images.
	//     type: boolean
	//     default: true
	//   - in: query
	//     name: force
	//     description: Force image removal (including containers using the images).
	//     type: boolean
	//   - in: query
	//     name: ignore
	//     description: Ignore if a specified image does not exist and do not throw an error.
	//     type: boolean
	//   - in: query
	//     name: lookupManifest
	//     description: Resolves to manifest list instead of image.
	//     type: boolean
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imagesRemoveResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/remove"), s.APIHandler(libpod.ImagesBatchRemove)).Methods(http.MethodDelete)
	// swagger:operation DELETE /libpod/images/{name} libpod ImageDeleteLibpod
	// ---
	// tags:
	//  - images
	// summary: Remove an image from the local storage.
	// description: Remove an image from the local storage.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: name or ID of image to remove
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: remove the image even if used by containers or has other tags
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imagesRemoveResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   409:
	//     $ref: '#/responses/conflictError'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}"), s.APIHandler(libpod.ImagesRemove)).Methods(http.MethodDelete)
	// swagger:operation POST /libpod/images/pull libpod ImagePullLibpod
	// ---
	// tags:
	//  - images
	// summary: Pull images
	// description: Pull one or more images from a container registry.
	// parameters:
	//   - in: query
	//     name: reference
	//     description: "Mandatory reference to the image (e.g., quay.io/image/name:tag)"
	//     type: string
	//   - in: query
	//     name: quiet
	//     description: "silences extra stream data on pull"
	//     type: boolean
	//     default: false
	//   - in: query
	//     name: credentials
	//     description: "username:password for the registry"
	//     type: string
	//   - in: query
	//     name: Arch
	//     description: Pull image for the specified architecture.
	//     type: string
	//   - in: query
	//     name: OS
	//     description: Pull image for the specified operating system.
	//     type: string
	//   - in: query
	//     name: Variant
	//     description: Pull image for the specified variant.
	//     type: string
	//   - in: query
	//     name: policy
	//     description: Pull policy, "always" (default), "missing", "newer", "never".
	//     type: string
	//   - in: query
	//     name: tlsVerify
	//     description: Require TLS verification.
	//     type: boolean
	//     default: true
	//   - in: query
	//     name: allTags
	//     description: Pull all tagged images in the repository.
	//     type: boolean
	//   - in: header
	//     name: X-Registry-Auth
	//     description: "base-64 encoded auth config. Must include the following four values: username, password, email and server address OR simply just an identity token."
	//     type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imagesPullResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/pull"), s.APIHandler(libpod.ImagesPull)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/images/prune libpod ImagePruneLibpod
	// ---
	// tags:
	//  - images
	// summary: Prune unused images
	// description: Remove images that are not being used by a container
	// parameters:
	//  - in: query
	//    name: all
	//    default: false
	//    type: boolean
	//    description: |
	//      Remove all images not in use by containers, not just dangling ones
	//  - in: query
	//    name: external
	//    default: false
	//    type: boolean
	//    description: |
	//      Remove images even when they are used by external containers (e.g, by build containers)
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
	//   200:
	//     $ref: "#/responses/imagesPruneLibpod"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/prune"), s.APIHandler(libpod.PruneImages)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/images/search libpod ImageSearchLibpod
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
	//    type: integer
	//    default: 25
	//    description: maximum number of results
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the images list. Available filters:
	//        - `is-automated=(true|false)`
	//        - `is-official=(true|false)`
	//        - `stars=<number>` Matches images that has at least 'number' stars.
	//  - in: query
	//    name: tlsVerify
	//    type: boolean
	//    default: true
	//    description: Require HTTPS and verify signatures when contacting registries.
	//  - in: query
	//    name: listTags
	//    type: boolean
	//    default: false
	//    description: list the available tags in the repository
	// produces:
	// - application/json
	// responses:
	//   200:
	//      $ref: "#/responses/registrySearchResponse"
	//   500:
	//      $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/search"), s.APIHandler(compat.SearchImages)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{name}/get libpod ImageGetLibpod
	// ---
	// tags:
	//  - images
	// summary: Export an image
	// description: Export an image
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: format
	//    type: string
	//    description: format for exported image
	//  - in: query
	//    name: compress
	//    type: boolean
	//    description: use compression on image
	// produces:
	// - application/x-tar
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/get"), s.APIHandler(libpod.ExportImage)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/export libpod ImageExportLibpod
	// ---
	// tags:
	//  - images
	// summary: Export multiple images
	// description: Export multiple images into a single object. Only `docker-archive` is currently supported.
	// parameters:
	//  - in: query
	//    name: format
	//    type: string
	//    description: format for exported image (only docker-archive is supported)
	//  - in: query
	//    name: references
	//    description: references to images to export
	//    type: array
	//    items:
	//      type: string
	//  - in: query
	//    name: compress
	//    type: boolean
	//    description: use compression on image
	//  - in: query
	//    name: ociAcceptUncompressedLayers
	//    type: boolean
	//    description: accept uncompressed layers when copying OCI images
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/export"), s.APIHandler(libpod.ExportImages)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/images/{name}/json libpod ImageInspectLibpod
	// ---
	// tags:
	//  - images
	// summary: Inspect an image
	// description: Obtain low-level information about an image
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/inspectImageResponseLibpod"
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/json"), s.APIHandler(libpod.GetImage)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/images/{name}/tag libpod ImageTagLibpod
	// ---
	// tags:
	//  - images
	// summary: Tag an image
	// description: Tag an image so that it becomes part of a repository.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
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
	//     $ref: '#/responses/badParamError'
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   409:
	//     $ref: '#/responses/conflictError'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/tag"), s.APIHandler(compat.TagImage)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/commit libpod ImageCommitLibpod
	// ---
	// tags:
	//  - containers
	// summary: Commit
	// description: Create a new image from a container
	// parameters:
	//  - in: query
	//    name: container
	//    type: string
	//    description: the name or ID of a container
	//    required: true
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
	//    type: boolean
	//    description: pause the container before committing it
	//  - in: query
	//    name: changes
	//    description: instructions to apply while committing in Dockerfile format (i.e. "CMD=/bin/foo")
	//    type: array
	//    items:
	//       type: string
	//  - in: query
	//    name: format
	//    type: string
	//    description: format of the image manifest and metadata (default "oci")
	// produces:
	// - application/json
	// responses:
	//   201:
	//     description: no error
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/commit"), s.APIHandler(libpod.CommitContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/images/{name}/untag libpod ImageUntagLibpod
	// ---
	// tags:
	//  - images
	// summary: Untag an image
	// description: Untag an image. If not repo and tag are specified, all tags are removed from the image.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: repo
	//    type: string
	//    description: the repository to untag
	//  - in: query
	//    name: tag
	//    type: string
	//    description: the name of the tag to untag
	// produces:
	// - application/json
	// responses:
	//   201:
	//     description: no error
	//   400:
	//     $ref: '#/responses/badParamError'
	//   404:
	//     $ref: '#/responses/imageNotFound'
	//   409:
	//     $ref: '#/responses/conflictError'
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/{name:.*}/untag"), s.APIHandler(libpod.UntagImage)).Methods(http.MethodPost)

	// swagger:operation GET /libpod/images/{name}/changes libpod ImageChangesLibpod
	// ---
	// tags:
	//   - images
	// summary: Report on changes to images's filesystem; adds, deletes or modifications.
	// description: |
	//   Returns which files in a images's filesystem have been added, deleted, or modified. The Kind of modification can be one of:
	//
	//   0: Modified
	//   1: Added
	//   2: Deleted
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the image
	//  - in: query
	//    name: parent
	//    type: string
	//    description: specify a second layer which is used to compare against it instead of the parent layer
	//  - in: query
	//    name: diffType
	//    type: string
	//    enum: [all, container, image]
	//    description: select what you want to match, default is all
	// responses:
	//   200:
	//     description: Array of Changes
	//     content:
	//       application/json:
	//       schema:
	//         $ref: "#/responses/Changes"
	//   404:
	//     $ref: "#/responses/containerNotFound"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/images/{name}/changes"), s.APIHandler(compat.Changes)).Methods(http.MethodGet)

	// swagger:operation POST /libpod/build libpod ImageBuildLibpod
	// ---
	// tags:
	//  - images
	// summary: Create image
	// description: Build an image from the given Dockerfile(s)
	// parameters:
	//  - in: query
	//    name: dockerfile
	//    type: string
	//    default: Dockerfile
	//    description: |
	//      Path within the build context to the `Dockerfile`.
	//      This is ignored if remote is specified and points to an external `Dockerfile`.
	//  - in: query
	//    name: t
	//    type: string
	//    default: latest
	//    description: A name and optional tag to apply to the image in the `name:tag` format.  If you omit the tag the default latest value is assumed. You can provide several t parameters.
	//  - in: query
	//    name: allplatforms
	//    type: boolean
	//    default: false
	//    description: |
	//      Instead of building for a set of platforms specified using the platform option, inspect the build's base images,
	//      and build for all of the platforms that are available.  Stages that use *scratch* as a starting point can not be inspected,
	//      so at least one non-*scratch* stage must be present for detection to work usefully.
	//  - in: query
	//    name: extrahosts
	//    type: string
	//    default:
	//    description: |
	//      TBD Extra hosts to add to /etc/hosts
	//      (As of version 1.xx)
	//  - in: query
	//    name: remote
	//    type: string
	//    default:
	//    description: |
	//      A Git repository URI or HTTP/HTTPS context URI.
	//      If the URI points to a single text file, the file’s contents are placed
	//      into a file called Dockerfile and the image is built from that file. If
	//      the URI points to a tarball, the file is downloaded by the daemon and the
	//      contents therein used as the context for the build. If the URI points to a
	//      tarball and the dockerfile parameter is also specified, there must be a file
	//      with the corresponding path inside the tarball.
	//      (As of version 1.xx)
	//  - in: query
	//    name: q
	//    type: boolean
	//    default: false
	//    description: |
	//      Suppress verbose build output
	//  - in: query
	//    name: nocache
	//    type: boolean
	//    default: false
	//    description: |
	//      Do not use the cache when building the image
	//      (As of version 1.xx)
	//  - in: query
	//    name: cachefrom
	//    type: string
	//    default:
	//    description: |
	//      JSON array of images used to build cache resolution
	//      (As of version 1.xx)
	//  - in: query
	//    name: pull
	//    type: boolean
	//    default: false
	//    description: |
	//      Attempt to pull the image even if an older image exists locally
	//      (As of version 1.xx)
	//  - in: query
	//    name: rm
	//    type: boolean
	//    default: true
	//    description: |
	//      Remove intermediate containers after a successful build
	//      (As of version 1.xx)
	//  - in: query
	//    name: forcerm
	//    type: boolean
	//    default: false
	//    description: |
	//      Always remove intermediate containers, even upon failure
	//      (As of version 1.xx)
	//  - in: query
	//    name: memory
	//    type: integer
	//    description: |
	//      Memory is the upper limit (in bytes) on how much memory running containers can use
	//      (As of version 1.xx)
	//  - in: query
	//    name: memswap
	//    type: integer
	//    description: |
	//      MemorySwap limits the amount of memory and swap together
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpushares
	//    type: integer
	//    description: |
	//      CPUShares (relative weight
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpusetcpus
	//    type: string
	//    description: |
	//      CPUSetCPUs in which to allow execution (0-3, 0,1)
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpuperiod
	//    type: integer
	//    description: |
	//      CPUPeriod limits the CPU CFS (Completely Fair Scheduler) period
	//      (As of version 1.xx)
	//  - in: query
	//    name: cpuquota
	//    type: integer
	//    description: |
	//      CPUQuota limits the CPU CFS (Completely Fair Scheduler) quota
	//      (As of version 1.xx)
	//  - in: query
	//    name: buildargs
	//    type: string
	//    default:
	//    description: |
	//      JSON map of string pairs denoting build-time variables.
	//      For example, the build argument `Foo` with the value of `bar` would be encoded in JSON as `["Foo":"bar"]`.
	//
	//      For example, buildargs={"Foo":"bar"}.
	//
	//      Note(s):
	//      * This should not be used to pass secrets.
	//      * The value of buildargs should be URI component encoded before being passed to the API.
	//
	//      (As of version 1.xx)
	//  - in: query
	//    name: shmsize
	//    type: integer
	//    default: 67108864
	//    description: |
	//      ShmSize is the "size" value to use when mounting an shmfs on the container's /dev/shm directory.
	//      Default is 64MB
	//      (As of version 1.xx)
	//  - in: query
	//    name: squash
	//    type: boolean
	//    default: false
	//    description: |
	//      Silently ignored.
	//      Squash the resulting images layers into a single layer
	//      (As of version 1.xx)
	//  - in: query
	//    name: labels
	//    type: string
	//    default:
	//    description: |
	//      JSON map of key, value pairs to set as labels on the new image
	//      (As of version 1.xx)
	//  - in: query
	//    name: layers
	//    type: boolean
	//    default: true
	//    description: |
	//      Cache intermediate layers during build.
	//      (As of version 1.xx)
	//  - in: query
	//    name: networkmode
	//    type: string
	//    default: bridge
	//    description: |
	//      Sets the networking mode for the run commands during build.
	//      Supported standard values are:
	//        * `bridge` limited to containers within a single host, port mapping required for external access
	//        * `host` no isolation between host and containers on this network
	//        * `none` disable all networking for this container
	//        * container:<nameOrID> share networking with given container
	//        ---All other values are assumed to be a custom network's name
	//      (As of version 1.xx)
	//  - in: query
	//    name: platform
	//    type: string
	//    default:
	//    description: |
	//      Platform format os[/arch[/variant]]
	//      (As of version 1.xx)
	//  - in: query
	//    name: target
	//    type: string
	//    default:
	//    description: |
	//      Target build stage
	//      (As of version 1.xx)
	//  - in: query
	//    name: outputs
	//    type: string
	//    default:
	//    description: |
	//      output configuration TBD
	//      (As of version 1.xx)
	//  - in: query
	//    name: httpproxy
	//    type: boolean
	//    default:
	//    description: |
	//      Inject http proxy environment variables into container
	//      (As of version 2.0.0)
	//  - in: query
	//    name: unsetenv
	//    description: Unset environment variables from the final image.
	//    type: array
	//    items:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: OK (As of version 1.xx)
	//     schema:
	//       type: object
	//       required:
	//         - stream
	//       properties:
	//         stream:
	//           type: string
	//           description: output from build process
	//           example: |
	//             (build details...)
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/build"), s.APIHandler(compat.BuildImage)).Methods(http.MethodPost)

	// swagger:operation POST /libpod/images/scp/{name} libpod ImageScpLibpod
	// ---
	// tags:
	//  - images
	// summary: Copy an image from one host to another
	// description: Copy an image from one host to another
	// parameters:
	//   - in: path
	//     name: name
	//     required: true
	//     description: source connection/image
	//     type: string
	//   - in: query
	//     name: destination
	//     required: false
	//     description: dest connection/image
	//     type: string
	//   - in: query
	//     name: quiet
	//     required: false
	//     description: quiet output
	//     type: boolean
	//     default: false
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/imagesScpResponseLibpod"
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: '#/responses/internalError'
	r.Handle(VersionedPath("/libpod/images/scp/{name:.*}"), s.APIHandler(libpod.ImageScp)).Methods(http.MethodPost)
	return nil
}
