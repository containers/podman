package system

import (
	"bufio"
	"context"
	"html/template"
	"os"
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	eventsDescription = "Monitor podman events"
	eventsCommand     = &cobra.Command{
		Use:   "events",
		Args:  validate.NoArgs,
		Short: "Show podman events",
		Long:  eventsDescription,
		RunE:  eventsCmd,
		Example: `podman events
  podman events --filter event=create
  podman events --since 1h30s`,
	}
)

var (
	eventOptions entities.EventsOptions
	eventFormat  string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: eventsCommand,
	})
	flags := eventsCommand.Flags()
	flags.StringArrayVar(&eventOptions.Filter, "filter", []string{}, "filter output")
	flags.StringVar(&eventFormat, "format", "", "format the output using a Go template")
	flags.BoolVar(&eventOptions.Stream, "stream", true, "stream new events; for testing only")
	flags.StringVar(&eventOptions.Since, "since", "", "show all events created since timestamp")
	flags.StringVar(&eventOptions.Until, "until", "", "show all events until timestamp")
	_ = flags.MarkHidden("stream")
}

func eventsCmd(cmd *cobra.Command, args []string) error {
	var (
		err         error
		eventsError error
		tmpl        *template.Template
	)
	if strings.Join(strings.Fields(eventFormat), "") == "{{json.}}" {
		eventFormat = formats.JSONString
	}
	if eventFormat != formats.JSONString {
		tmpl, err = template.New("events").Parse(eventFormat)
		if err != nil {
			return err
		}
	}
	if len(eventOptions.Since) > 0 || len(eventOptions.Until) > 0 {
		eventOptions.FromStart = true
	}
	eventChannel := make(chan *events.Event)
	eventOptions.EventChan = eventChannel

	go func() {
		eventsError = registry.ContainerEngine().Events(context.Background(), eventOptions)
	}()
	if eventsError != nil {
		return eventsError
	}

	w := bufio.NewWriter(os.Stdout)
	for event := range eventChannel {
		switch {
		case eventFormat == formats.JSONString:
			jsonStr, err := event.ToJSONString()
			if err != nil {
				return errors.Wrapf(err, "unable to format json")
			}
			if _, err := w.Write([]byte(jsonStr)); err != nil {
				return err
			}
		case len(eventFormat) > 0:
			if err := tmpl.Execute(w, event); err != nil {
				return err
			}
		default:
			if _, err := w.Write([]byte(event.ToHumanReadable())); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}
