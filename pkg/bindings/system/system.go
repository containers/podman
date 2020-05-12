package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Events allows you to monitor libdpod related events like container creation and
// removal.  The events are then passed to the eventChan provided. The optional cancelChan
// can be used to cancel the read of events and close down the HTTP connection.
func Events(ctx context.Context, eventChan chan (entities.Event), cancelChan chan bool, since, until *string, filters map[string][]string) error {
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
		e := entities.Event{}
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

// Prune removes all unused system data.
func Prune(ctx context.Context, all, volumes *bool) (*entities.SystemPruneReport, error) {
	var (
		report entities.SystemPruneReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if all != nil {
		params.Set("All", strconv.FormatBool(*all))
	}
	if volumes != nil {
		params.Set("Volumes", strconv.FormatBool(*volumes))
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/system/prune", params)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

func Version(ctx context.Context) (*entities.SystemVersionReport, error) {
	var report entities.SystemVersionReport
	var component entities.ComponentVersion

	version, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	report.Client = &version

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}

	if err = response.Process(&component); err != nil {
		return nil, err
	}
	f, _ := strconv.ParseFloat(component.APIVersion, 64)
	b, _ := time.Parse(time.RFC3339, component.BuildTime)
	report.Server = &define.Version{
		RemoteAPIVersion: int64(f),
		Version:          component.Version.Version,
		GoVersion:        component.GoVersion,
		GitCommit:        component.GitCommit,
		Built:            b.Unix(),
		OsArch:           fmt.Sprintf("%s/%s", component.Os, component.Arch),
	}
	return &report, err
}

// Reset removes all unused system data.
func Reset(ctx context.Context) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/system/reset", nil)
	if err != nil {
		return err
	}
	return response.Process(response)
}
