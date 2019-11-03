package serviceapi


import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/mux"
)

func registerInfoHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/info"), serviceHandler(info))
	return nil
}

func info(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	infoData, err := runtime.Info()
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to obtain the system information: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}
	info, err := InfoDataToInfo(infoData)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to convert system information to API information: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}

	buffer, err := json.Marshal(info)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("Failed to convert API images to json: %s", err.Error()),
			http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, string(buffer))
}
