package serviceapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/libpod/libpod"
)

func images(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		http.Error(w,
			fmt.Sprintf("%s is not a supported Content-Type", r.Header.Get("Content-Type")),
			http.StatusUnsupportedMediaType)
		return
	}

	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var summaries []*ImageSummary
	for _, img := range images {
		i, err := ImageToImageSummary(img)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		summaries = append(summaries, i)
	}

	buffer, err := json.Marshal(summaries)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, string(buffer))
}
