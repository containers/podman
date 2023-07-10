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
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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

	statusWritten := false
	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)

loop: // break out of for/select infinite` loop
	for {
		select {
		case <-r.Context().Done():
			break loop
		default:
			output, err := c.Top(strings.Split(query.PsArgs, ","))
			if err != nil {
				if !statusWritten {
					utils.InternalServerError(w, err)
				} else {
					logrus.Errorf("From %s %q : %v", r.Method, r.URL, err)
				}
				break loop
			}

			if len(output) > 0 {
				body := handlers.ContainerTopOKBody{}
				body.Titles = utils.PSTitles(output[0])

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
					if !statusWritten {
						utils.InternalServerError(w, err)
					} else {
						logrus.Errorf("From %s %q : %v", r.Method, r.URL, err)
					}
					break loop
				}
				// after the first write we can no longer send a different status code
				statusWritten = true
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
