package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerImagesHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/images/json"), serviceHandler(getImages)).Methods("GET")
	r.Handle(versionedPath("/images/{name:..*}/json"), serviceHandler(image))
	return nil
}

func image(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]
	image, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		apiError(w, fmt.Sprintf("Image '%s' not found", name), http.StatusNotFound)
		return
	}

	info, err := image.Inspect(context.Background())
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
