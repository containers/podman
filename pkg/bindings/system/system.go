package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Events allows you to monitor libdpod related events like container creation and
// removal.  The events are then passed to the eventChan provided. The optional cancelChan
// can be used to cancel the read of events and close down the HTTP connection.
func Events(ctx context.Context, eventChan chan entities.Event, cancelChan chan bool, options *EventsOptions) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params, err := options.ToParams()
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/events", params, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if cancelChan != nil {
		go func() {
			<-cancelChan
			err = response.Body.Close()
			logrus.Error(errors.Wrap(err, "unable to close event response body"))
		}()
	}

	dec := json.NewDecoder(response.Body)
	for err = (error)(nil); err == nil; {
		var e = entities.Event{}
		err = dec.Decode(&e)
		if err == nil {
			eventChan <- e
		}
	}
	close(eventChan)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF):
		return nil
	default:
		return errors.Wrap(err, "unable to decode event response")
	}
}

// Prune removes all unused system data.
func Prune(ctx context.Context, options *PruneOptions) (*entities.SystemPruneReport, error) {
	var (
		report entities.SystemPruneReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/system/prune", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}

func Version(ctx context.Context, options *VersionOptions) (*entities.SystemVersionReport, error) {
	var (
		component entities.ComponentVersion
		report    entities.SystemVersionReport
	)
	if options == nil {
		options = new(VersionOptions)
	}
	_ = options
	version, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	report.Client = &version

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/version", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err = response.Process(&component); err != nil {
		return nil, err
	}

	b, _ := time.Parse(time.RFC3339, component.BuildTime)
	report.Server = &define.Version{
		APIVersion: component.APIVersion,
		Version:    component.Version.Version,
		GoVersion:  component.GoVersion,
		GitCommit:  component.GitCommit,
		BuiltTime:  time.Unix(b.Unix(), 0).Format(time.ANSIC),
		Built:      b.Unix(),
		OsArch:     fmt.Sprintf("%s/%s", component.Os, component.Arch),
		Os:         component.Os,
	}

	for _, c := range component.Components {
		if c.Name == "Podman Engine" {
			report.Server.APIVersion = c.Details["APIVersion"]
		}
	}
	return &report, err
}

// DiskUsage returns information about image, container, and volume disk
// consumption
func DiskUsage(ctx context.Context, options *DiskOptions) (*entities.SystemDfReport, error) {
	var report entities.SystemDfReport
	if options == nil {
		options = new(DiskOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/system/df", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}
