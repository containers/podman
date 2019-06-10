package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	eventsCommand     cliconfig.EventValues
	eventsDescription = "Monitor podman events"
	_eventsCommand    = &cobra.Command{
		Use:   "events",
		Args:  noSubArgs,
		Short: "Show podman events",
		Long:  eventsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			eventsCommand.InputArgs = args
			eventsCommand.GlobalFlags = MainGlobalOpts
			eventsCommand.Remote = remoteclient
			return eventsCmd(&eventsCommand)
		},
		Example: `podman events
  podman events --filter event=create
  podman events --since 1h30s`,
	}
)

func init() {
	eventsCommand.Command = _eventsCommand
	eventsCommand.SetUsageTemplate(UsageTemplate())
	flags := eventsCommand.Flags()
	flags.StringArrayVar(&eventsCommand.Filter, "filter", []string{}, "filter output")
	flags.StringVar(&eventsCommand.Format, "format", "", "format the output using a Go template")
	flags.BoolVar(&eventsCommand.Stream, "stream", true, "stream new events; for testing only")
	flags.StringVar(&eventsCommand.Since, "since", "", "show all events created since timestamp")
	flags.StringVar(&eventsCommand.Until, "until", "", "show all events until timestamp")
	flags.MarkHidden("stream")
}

func eventsCmd(c *cliconfig.EventValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	return runtime.Events(c)
}
