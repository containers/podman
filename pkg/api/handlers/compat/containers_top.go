package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func TopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	psArgs := "-ef"
	if utils.IsLibpodRequest(r) {
		psArgs = ""
	}
	query := struct {
		Delay  int    `schema:"delay"`
		PsArgs string `schema:"ps_args"`
		Stream bool   `schema:"stream"`
	}{
		Delay:  5,
		PsArgs: psArgs,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if query.Delay < 1 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("\"delay\" parameter of value %d < 1", query.Delay))
		return
	}

	name := utils.GetName(r)
	c, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// We are committed now - all errors logged but not reported to client, ship has sailed
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	encoder := json.NewEncoder(w)

loop: // break out of for/select infinite` loop
	for {
		select {
		case <-r.Context().Done():
			break loop
		default:
			output, err := c.Top([]string{query.PsArgs})
			if err != nil {
				logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
				break loop
			}

			if len(output) > 0 {
				body := handlers.ContainerTopOKBody{}
				body.Titles = strings.Split(output[0], "\t")
				for i := range body.Titles {
					body.Titles[i] = strings.TrimSpace(body.Titles[i])
				}

				for _, line := range output[1:] {
					process := strings.Split(line, "\t")
					for i := range process {
						process[i] = strings.TrimSpace(process[i])
					}
					body.Processes = append(body.Processes, process)
				}

				if err := encoder.Encode(body); err != nil {
					logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
					break loop
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}

			if query.Stream {
				time.Sleep(time.Duration(query.Delay) * time.Second)
			} else {
				break loop
			}
		}
	}
}
