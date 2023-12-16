package system

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/storage/pkg/stringid"
	"github.com/spf13/cobra"
)

var (
	eventsDescription = `Monitor podman system events.

  By default, streaming mode is used, printing new events as they occur.  Previous events can be listed via --since and --until.`
	eventsCommand = &cobra.Command{
		Use:               "events [options]",
		Args:              validate.NoArgs,
		Short:             "Show podman system events",
		Long:              eventsDescription,
		RunE:              eventsCmd,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman events
  podman events --filter event=create
  podman events --format {{.Image}}
  podman events --since 1h30s`,
	}

	systemEventsCommand = &cobra.Command{
		Args:              eventsCommand.Args,
		Use:               eventsCommand.Use,
		Short:             eventsCommand.Short,
		Long:              eventsCommand.Long,
		RunE:              eventsCommand.RunE,
		ValidArgsFunction: eventsCommand.ValidArgsFunction,
		Example:           `podman system events`,
	}
)

var (
	eventOptions entities.EventsOptions
	eventFormat  string
	noTrunc      bool
)

type Event struct {
	// containerExitCode is for storing the exit code of a container which can
	// be used for "internal" event notification
	ContainerExitCode *int `json:",omitempty"`
	// ID can be for the container, image, volume, etc
	ID string `json:",omitempty"`
	// Image used where applicable
	Image string `json:",omitempty"`
	// Name where applicable
	Name string `json:",omitempty"`
	// Network is the network name in a network event
	Network string `json:"network,omitempty"`
	// Status describes the event that occurred
	Status events.Status
	// Time the event occurred
	Time int64 `json:"time,omitempty"`
	// timeNano the event occurred in nanoseconds
	TimeNano int64 `json:"timeNano,omitempty"`
	// Type of event that occurred
	Type events.Type
	// Health status of the current container
	HealthStatus string `json:"health_status,omitempty"`

	events.Details
}

func newEventFromLibpodEvent(e events.Event) Event {
	return Event{
		ContainerExitCode: e.ContainerExitCode,
		ID:                e.ID,
		Image:             e.Image,
		Name:              e.Name,
		Network:           e.Network,
		Status:            e.Status,
		Time:              e.Time.Unix(),
		Type:              e.Type,
		HealthStatus:      e.HealthStatus,
		Details:           e.Details,
		TimeNano:          e.Time.UnixNano(),
	}
}

func (e *Event) ToJSONString() (string, error) {
	b, err := jsonencoding.Marshal(e)
	return string(b), err
}

func (e *Event) ToHumanReadable(truncate bool) string {
	if e == nil {
		return ""
	}
	var humanFormat string
	id := e.ID
	if truncate {
		id = stringid.TruncateID(id)
	}

	timeUnix := time.Unix(0, e.TimeNano)

	switch e.Type {
	case events.Container, events.Pod:
		humanFormat = fmt.Sprintf("%s %s %s %s (image=%s, name=%s", timeUnix, e.Type, e.Status, id, e.Image, e.Name)
		if e.PodID != "" {
			humanFormat += fmt.Sprintf(", pod_id=%s", e.PodID)
		}
		if e.HealthStatus != "" {
			humanFormat += fmt.Sprintf(", health_status=%s", e.HealthStatus)
		}
		// check if the container has labels and add it to the output
		if len(e.Attributes) > 0 {
			for k, v := range e.Attributes {
				humanFormat += fmt.Sprintf(", %s=%s", k, v)
			}
		}
		humanFormat += ")"
	case events.Network:
		humanFormat = fmt.Sprintf("%s %s %s %s (container=%s, name=%s)", timeUnix, e.Type, e.Status, id, id, e.Network)
	case events.Image:
		humanFormat = fmt.Sprintf("%s %s %s %s %s", timeUnix, e.Type, e.Status, id, e.Name)
	case events.System:
		if e.Name != "" {
			humanFormat = fmt.Sprintf("%s %s %s %s", timeUnix, e.Type, e.Status, e.Name)
		} else {
			humanFormat = fmt.Sprintf("%s %s %s", timeUnix, e.Type, e.Status)
		}
	case events.Volume, events.Machine:
		humanFormat = fmt.Sprintf("%s %s %s %s", timeUnix, e.Type, e.Status, e.Name)
	}
	return humanFormat
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemEventsCommand,
		Parent:  systemCmd,
	})
	eventsFlags(systemEventsCommand)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: eventsCommand,
	})
	eventsFlags(eventsCommand)
}

func eventsFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	filterFlagName := "filter"
	flags.StringArrayVarP(&eventOptions.Filter, filterFlagName, "f", []string{}, "filter output")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteEventFilter)

	formatFlagName := "format"
	flags.StringVar(&eventFormat, formatFlagName, "", "format the output using a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&Event{}))

	flags.BoolVar(&eventOptions.Stream, "stream", true, "stream events and do not exit when returning the last known event")

	sinceFlagName := "since"
	flags.StringVar(&eventOptions.Since, sinceFlagName, "", "show all events created since timestamp")
	_ = cmd.RegisterFlagCompletionFunc(sinceFlagName, completion.AutocompleteNone)

	flags.BoolVar(&noTrunc, "no-trunc", true, "do not truncate the output")

	untilFlagName := "until"
	flags.StringVar(&eventOptions.Until, untilFlagName, "", "show all events until timestamp")
	_ = cmd.RegisterFlagCompletionFunc(untilFlagName, completion.AutocompleteNone)
}

func eventsCmd(cmd *cobra.Command, _ []string) error {
	if len(eventOptions.Since) > 0 || len(eventOptions.Until) > 0 {
		eventOptions.FromStart = true
	}
	eventChannel := make(chan *events.Event, 1)
	eventOptions.EventChan = eventChannel
	errChannel := make(chan error)

	var (
		rpt    *report.Formatter
		doJSON bool
	)

	if cmd.Flags().Changed("format") {
		doJSON = report.IsJSON(eventFormat)
		if !doJSON {
			var err error
			// Use OriginUnknown so it does not add an extra range since it
			// will only be called for each single element and not a slice.
			rpt, err = report.New(os.Stdout, cmd.Name()).Parse(report.OriginUnknown, eventFormat)
			if err != nil {
				return err
			}
		}
	}

	go func() {
		err := registry.ContainerEngine().Events(context.Background(), eventOptions)
		errChannel <- err
	}()

	for {
		select {
		case event, ok := <-eventChannel:
			if !ok {
				// channel was closed we can exit
				return nil
			}
			e := newEventFromLibpodEvent(*event)
			switch {
			case doJSON:
				jsonStr, err := e.ToJSONString()
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
			case cmd.Flags().Changed("format"):
				if err := rpt.Execute(event); err != nil {
					return err
				}
			default:
				fmt.Println(e.ToHumanReadable(!noTrunc))
			}
		case err := <-errChannel:
			// only exit in case of an error,
			// otherwise keep reading events until the event channel is closed
			if err != nil {
				return err
			}
		}
	}
}
