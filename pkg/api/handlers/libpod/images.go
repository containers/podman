package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// GetImages list images
// digests bool
// filters []string json encoded map of string to stringslice
// all

// LoadImage
// quiet bool

// Image Prune
//  filter
// deleted images and space reclaimed

// Rmi
// force
//noprune future -> false
// { untagged and deleted}

// Inspect
// no input

// Tag
// repo, tag string
// returns codes 201

// Commit
// author string
// "container"
// repo string
// tag string
// message
// pause bool
// changes []string

// create

func ImageExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]

	_, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		handlers.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	handlers.WriteResponse(w, http.StatusOK, "Ok")
}

func ImageTree(w http.ResponseWriter, r *http.Request) {
	// tree is a bit of a mess ... logic is in adapter and therefore not callable from here. needs rework

	//name := mux.Vars(r)["name"]
	//_, layerInfoMap, _, err := s.Runtime.Tree(name)
	//if err != nil {
	//	Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to find image information for %q", name))
	//	return
	//}
	//	it is not clear to me how to deal with this given all the processing of the image
	// is in main.  we need to discuss how that really should be and return something useful.
	handlers.UnsupportedHandler(w, r)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	newImage, err := handlers.GetImage(r, name)
	if err != nil {
		handlers.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	inspect, err := newImage.Inspect(r.Context())
	if err != nil {
		handlers.Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "failed in inspect image %s", inspect.ID))
		return
	}
	handlers.WriteResponse(w, http.StatusOK, inspect)

}
func GetImages(w http.ResponseWriter, r *http.Request) {}

func LoadImage(w http.ResponseWriter, r *http.Request)   {}
func PruneImages(w http.ResponseWriter, r *http.Request) {}
func ExportImage(w http.ResponseWriter, r *http.Request) {}
