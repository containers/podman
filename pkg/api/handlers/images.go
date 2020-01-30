package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func HistoryImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	var allHistory []HistoryResponse

	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return

	}
	history, err := newImage.History(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, h := range history {
		l := HistoryResponse{
			ID:        h.ID,
			Created:   h.Created.UnixNano(),
			CreatedBy: h.CreatedBy,
			Tags:      h.Tags,
			Size:      h.Size,
			Comment:   h.Comment,
		}
		allHistory = append(allHistory, l)
	}
	utils.WriteResponse(w, http.StatusOK, allHistory)
}

func TagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /v1.xx/images/(name)/tag
	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.TagImage(tagName); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}

func RemoveImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		noPrune bool
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	muxVars := mux.Vars(r)
	if _, found := muxVars["noprune"]; found {
		if query.noPrune {
			utils.UnSupportedParameter("noprune")
		}
	}
	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}

	force := false
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusBadRequest, err)
			return
		}
	}
	_, err = runtime.RemoveImage(r.Context(), newImage, force)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	// TODO
	// This will need to be fixed for proper response, like Deleted: and Untagged:
	m := make(map[string]string)
	m["Deleted"] = newImage.ID()
	foo := []map[string]string{}
	foo = append(foo, m)
	utils.WriteResponse(w, http.StatusOK, foo)

}
func GetImage(r *http.Request, name string) (*image.Image, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	return runtime.ImageRuntime().NewFromLocal(name)
}

func SaveFromBody(f *os.File, r *http.Request) error { // nolint
	if _, err := io.Copy(f, r.Body); err != nil {
		return err
	}
	return f.Close()
}

func SearchImages(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Term    string              `json:"term"`
		Limit   int                 `json:"limit"`
		Filters map[string][]string `json:"filters"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	// TODO filters are a bit undefined here in terms of what exactly the input looks
	// like. We need to understand that a bit more.
	options := image.SearchOptions{
		Filter: image.SearchFilter{},
		Limit:  query.Limit,
	}
	results, err := image.SearchImages(query.Term, options)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, results)
}
