package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/gorilla/mux"
)

func registerImagesHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/images/json"), serviceHandler(getImages)).Methods("GET")
	r.Handle(versionedPath("/images/{name:..*}/json"), serviceHandler(image))
	r.Handle(versionedPath("/images/{name:..*}/tag"), serviceHandler(tagImage))
	r.Handle(versionedPath("/images/create"), serviceHandler(createImage))
	return nil
}

// this is temporary and will be removed by Brent ASAP! Like  next PR
func sendJSONResponse(w http.ResponseWriter, msg []byte) error {
	w.Header().Set("Content-Type", "application/json")
	_, err := io.WriteString(w, string(msg))
	return err
}

func createImage(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	/*
	   fromImage – Name of the image to pull. The name may include a tag or digest. This parameter may only be used when pulling an image. The pull is cancelled if the HTTP connection is closed.
	   fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	   repo – Repository name given to an image when it is imported. The repo may include a tag. This parameter may only be used when importing an image.
	   tag – Tag or digest. If empty when pulling an image, this causes all tags for the given image to be pulled.
	*/
	ctx := context.Background()
	fromImage := r.Form.Get("fromImage")
	// TODO
	// We are eating the output right now because we havent taked about how to deal with multiple reponses yet
	_, err := runtime.ImageRuntime().New(ctx, fromImage, "", "", nil, &image2.DockerRegistryOptions{}, image2.SigningOptions{}, nil, util.PullImageAlways)
	if err != nil {
		apiError(w, fmt.Sprintf("unable to pull%s: %s", fromImage, err.Error()), http.StatusInternalServerError)
		return
	}
	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}

func tagImage(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/images/(name)/tag
	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		noSuchImageError(w, name)
		return
	}
	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		apiError(w, fmt.Sprint("repo parameter is required"), http.StatusBadRequest)
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.TagImage(tagName); err != nil {
		apiError(w, fmt.Sprintf("unable to tag %s: %s", name, err.Error()), http.StatusInternalServerError)
		return
	}
	// Success is a 201?
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "")
	return
}

func image(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		noSuchImageError(w, name)
		return
	}
	ctx := context.Background()

	switch r.Method {
	case "DELETE":
		force := false
		if len(r.Form.Get("force")) > 0 {
			force, err = strconv.ParseBool(r.Form.Get("force"))
			if err != nil {
				apiError(w, fmt.Sprintf("unable to parse value for force: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}
		r, err := runtime.RemoveImage(ctx, newImage, force)
		if err != nil {
			apiError(w, fmt.Sprintf("unable to delete %s: %s", name, err.Error()), http.StatusInternalServerError)
			return
		}
		// TODO
		// This will need to be fixed for proper response, like Deleted: and Untagged:
		buffer, _ := json.Marshal(r)
		sendJSONResponse(w, buffer)
		return
	}

	info, err := newImage.Inspect(context.Background())
	if err != nil {
		apiError(w, fmt.Sprintf("Failed to inspect Image '%s'", name), http.StatusInternalServerError)
		return
	}

	inspect, err := ImageDataToImageInspect(info)
	buffer, err := json.Marshal(inspect)
	if err != nil {
		apiError(w,
			fmt.Sprintf("Failed to convert API ImageInspect '%s' to json: %s", inspect.ID, err.Error()),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(buffer))
}

func getImages(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	query := struct {
		all     bool
		filters string
		digests bool
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := r.ParseForm(); err != nil {
		apiError(w, fmt.Sprintf("Failed to parse input: %s", err.Error()), http.StatusBadRequest)
		return
	}

	var err error
	t := r.Form.Get("all")
	if t != "" {
		query.all, err = strconv.ParseBool(t)
		if err != nil {
			apiError(w, fmt.Sprintf("Failed to parse 'all' parameter %s: %s", t, err.Error()), http.StatusBadRequest)
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
			apiError(w, fmt.Sprintf("Failed to parse 'digests' parameter %s: %s", t, err.Error()), http.StatusBadRequest)
			return
		}
	}

	// FIXME placeholder until filters are implemented
	_ = query

	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		apiError(w,
			fmt.Sprintf("Failed to obtain the list of images from storage: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	var summaries []*ImageSummary
	for _, img := range images {
		i, err := ImageToImageSummary(img)
		if err != nil {
			apiError(w,
				fmt.Sprintf("Failed to convert storage image '%s' to API image: %s", img.ID(), err.Error()),
				http.StatusInternalServerError)
			return
		}
		summaries = append(summaries, i)
	}

	buffer, err := json.Marshal(summaries)
	if err != nil {
		apiError(w,
			fmt.Sprintf("Failed to convert API images to json: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(buffer))
}
