package system

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/domain/entities"
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
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&events.Event{}))

	flags.BoolVar(&eventOptions.Stream, "stream", true, "stream new events; for testing only")

	sinceFlagName := "since"
	flags.StringVar(&eventOptions.Since, sinceFlagName, "", "show all events created since timestamp")
	_ = cmd.RegisterFlagCompletionFunc(sinceFlagName, completion.AutocompleteNone)

	flags.BoolVar(&noTrunc, "no-trunc", true, "do not truncate the output")

	untilFlagName := "until"
	flags.StringVar(&eventOptions.Until, untilFlagName, "", "show all events until timestamp")
	_ = cmd.RegisterFlagCompletionFunc(untilFlagName, completion.AutocompleteNone)

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
			switch {
			case doJSON:
				jsonStr, err := event.ToJSONString()
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
			case cmd.Flags().Changed("format"):
				if err := rpt.Execute(event); err != nil {
					return err
				}
			default:
				fmt.Println(event.ToHumanReadable(!noTrunc))
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
