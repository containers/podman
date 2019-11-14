package serviceapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/images/json"), s.serviceHandler(s.getImages)).Methods("GET")
	r.Handle(versionedPath("/images/{name:..*}"), s.serviceHandler(s.removeImage)).Methods("DELETE")
	r.Handle(versionedPath("/images/{name:..*}/json"), s.serviceHandler(s.image))
	r.Handle(versionedPath("/images/{name:..*}/tag"), s.serviceHandler(s.tagImage)).Methods("POST")
	r.Handle(versionedPath("/images/create"), s.serviceHandler(s.createImage)).Methods("POST").Queries("fromImage", "{fromImage}")
	return nil
}

func (s *APIServer) createImage(w http.ResponseWriter, r *http.Request) {
	/*
	   fromImage – Name of the image to pull. The name may include a tag or digest. This parameter may only be used when pulling an image. The pull is cancelled if the HTTP connection is closed.
	   fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	   repo – Repository name given to an image when it is imported. The repo may include a tag. This parameter may only be used when importing an image.
	   tag – Tag or digest. If empty when pulling an image, this causes all tags for the given image to be pulled.
	*/
	fromImage := r.Form.Get("fromImage")

	tag := r.Form.Get("tag")
	if tag != "" {
		fromImage = fmt.Sprintf("%s:%s", fromImage, tag)
	}

	// TODO
	// We are eating the output right now because we haven't talked about how to deal with multiple responses yet
	img, err := s.Runtime.ImageRuntime().New(s.Context, fromImage, "", "", nil, &image2.DockerRegistryOptions{}, image2.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	s.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Error          string            `json:"error"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"`
	}{
		Status:         fmt.Sprintf("pulling image (%s) from %s", img.Tag, strings.Join(img.Names(), ", ")),
		ProgressDetail: map[string]string{},
		Id:             img.ID(),
	})
}

func (s *APIServer) tagImage(w http.ResponseWriter, r *http.Request) {
	// /v1.xx/images/(name)/tag
	name := mux.Vars(r)["name"]
	newImage, err := s.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		imageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.TagImage(tagName); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusCreated, "")
}

func (s *APIServer) removeImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	newImage, err := s.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		imageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}

	force := false
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, err)
			return
		}
	}
	id, err := s.Runtime.RemoveImage(s.Context, newImage, force)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	// TODO
	// This will need to be fixed for proper response, like Deleted: and Untagged:
	s.WriteResponse(w, http.StatusOK, struct {
		Deleted string `json:"Deleted"`
	}{
		Deleted: id,
	})

}
func (s *APIServer) image(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	newImage, err := s.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	info, err := newImage.Inspect(context.Background())
	if err != nil {
		Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "Failed to inspect image %s", name))
		return
	}

	inspect, err := ImageDataToImageInspect(info)
	if err != nil {
		Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "Failed to convert ImageData to ImageInspect '%s'", inspect.ID))
		return
	}

	s.WriteResponse(w, http.StatusOK, inspect)
}

func (s *APIServer) getImages(w http.ResponseWriter, r *http.Request) {
	query := struct {
		all     bool
		filters string
		digests bool
	}{
		// This is where you can override the golang default value for one of fields
	}

	var err error
	t := r.Form.Get("all")
	if t != "" {
		query.all, err = strconv.ParseBool(t)
		if err != nil {
			Error(w, "Server error", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse 'all' parameter %s", t))
			return
		}
	}

	// TODO How do we want to process filters
	t = r.Form.Get("filters")
	if t != "" {
		query.filters = t
	}

	t = r.Form.Get("digests")
	if t != "" {
		query.digests, err = strconv.ParseBool(t)
		if err != nil {
			Error(w, "Server error", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse 'digests' parameter %s", t))
			return
		}
	}

	// FIXME placeholder until filters are implemented
	_ = query

	images, err := s.Runtime.ImageRuntime().GetImages()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain the list of images from storage"))
		return
	}

	var summaries []*ImageSummary
	for _, img := range images {
		i, err := ImageToImageSummary(img)
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to convert storage image '%s' to API image"))
			return
		}
		summaries = append(summaries, i)
	}

	s.WriteResponse(w, http.StatusOK, summaries)
}
