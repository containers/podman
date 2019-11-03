package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerImagesHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/images/json"), serviceHandler(getImages))
	r.Handle(versionedPath("/images/{name:..*}/json"), serviceHandler(inspectImages))
	return nil
}
func inspectImages(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/images/(name)
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		http.Error(w,
			fmt.Sprintf("%s is not a supported Content-Type", r.Header.Get("Content-Type")),
			http.StatusUnsupportedMediaType)
		return
	}

	name := mux.Vars(r)["name"]
	image, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Image '%s' not found", name), http.StatusNotFound)
		return
	}

	info, err := image.Inspect(context.Background())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to inspect Image '%s'", name), http.StatusInternalServerError)
		return
	}

	inspect, err := ImageDataToImageInspect(info)
	buffer, err := json.Marshal(inspect)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to convert API ImageInspect '%s' to json: %s", inspect.ID, err.Error()),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, string(buffer))
}

func getImages(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/images/json

	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to obtain the list of images from storage: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	var summaries []*ImageSummary
	for _, img := range images {
		i, err := ImageToImageSummary(img)
		if err != nil {
			http.Error(w,
				fmt.Sprintf("Failed to convert storage image '%s' to API image: %s", img.ID(), err.Error()),
				http.StatusInternalServerError)
			return
		}
		summaries = append(summaries, i)
	}

	buffer, err := json.Marshal(summaries)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to convert API images to json: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, string(buffer))
}
