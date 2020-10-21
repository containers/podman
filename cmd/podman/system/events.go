package system

import (
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	eventsDescription = `Monitor podman events.

  By default, streaming mode is used, printing new events as they occur.  Previous events can be listed via --since and --until.`
	eventsCommand = &cobra.Command{
		Use:   "events [options]",
		Args:  validate.NoArgs,
		Short: "Show podman events",
		Long:  eventsDescription,
		RunE:  eventsCmd,
		Example: `podman events
  podman events --filter event=create
  podman events --format {{.Image}}
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

func eventsCmd(cmd *cobra.Command, _ []string) error {
	if len(eventOptions.Since) > 0 || len(eventOptions.Until) > 0 {
		eventOptions.FromStart = true
	}
	eventChannel := make(chan *events.Event, 1)
	eventOptions.EventChan = eventChannel
	errChannel := make(chan error)

	var (
		tmpl   *template.Template
		doJSON bool
	)

	if cmd.Flags().Changed("format") {
		doJSON = report.IsJSON(eventFormat)
		if !doJSON {
			var err error
			tmpl, err = template.New("events").Parse(eventFormat)
			if err != nil {
				return err
			}
		}
	}

	go func() {
		err := registry.ContainerEngine().Events(context.Background(), eventOptions)
		errChannel <- err
	}()

	for event := range eventChannel {
		switch {
		case event == nil:
			// no-op
		case doJSON:
			jsonStr, err := event.ToJSONString()
			if err != nil {
				return errors.Wrapf(err, "unable to format json")
			}
			fmt.Println(jsonStr)
		case cmd.Flags().Changed("format"):
			if err := tmpl.Execute(os.Stdout, event); err != nil {
				return err
			}
			fmt.Println("")
		default:
			fmt.Println(event.ToHumanReadable())
		}
	}

	return <-errChannel
}
