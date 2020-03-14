package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Events allows you to monitor libdpod related events like container creation and
// removal.  The events are then passed to the eventChan provided. The optional cancelChan
// can be used to cancel the read of events and close down the HTTP connection.
func Events(ctx context.Context, eventChan chan (handlers.Event), cancelChan chan bool, since, until *string, filters map[string][]string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if since != nil {
		params.Set("since", *since)
	}
	if until != nil {
		params.Set("until", *until)
	}
	if filters != nil {
		filterString, err := bindings.FiltersToString(filters)
		if err != nil {
			return errors.Wrap(err, "invalid filters")
		}
		params.Set("filters", filterString)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/events", params)
	if err != nil {
		return err
	}
	if cancelChan != nil {
		go func() {
			<-cancelChan
			err = response.Body.Close()
			logrus.Error(errors.Wrap(err, "unable to close event response body"))
		}()
	}
	dec := json.NewDecoder(response.Body)
	for {
		e := handlers.Event{}
		if err := dec.Decode(&e); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "unable to decode event response")
		}
		eventChan <- e
	}
	return nil
}
