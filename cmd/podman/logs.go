package main

import (
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	logsCommand     cliconfig.LogsValues
	logsDescription = `Retrieves logs for one or more containers.

  This does not guarantee execution order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs.
`
	_logsCommand = &cobra.Command{
		Use:   "logs [flags] CONTAINER [CONTAINER...]",
		Short: "Fetch the logs of a container",
		Long:  logsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			logsCommand.InputArgs = args
			logsCommand.GlobalFlags = MainGlobalOpts
			return logsCmd(&logsCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && logsCommand.Latest {
				return errors.New("no containers can be specified when using 'latest'")
			}
			if !logsCommand.Latest && len(args) < 1 {
				return errors.New("specify at least one container name or ID to log")
			}
			return nil
		},
		Example: `podman logs ctrID
  podman logs --tail 2 mywebserver
  podman logs --follow=true --since 10m ctrID
  podman logs mywebserver mydbserver`,
	}
)

func init() {
	logsCommand.Command = _logsCommand
	logsCommand.SetHelpTemplate(HelpTemplate())
	logsCommand.SetUsageTemplate(UsageTemplate())
	flags := logsCommand.Flags()
	flags.BoolVar(&logsCommand.Details, "details", false, "Show extra details provided to the logs")
	flags.BoolVarP(&logsCommand.Follow, "follow", "f", false, "Follow log output.  The default is false")
	flags.BoolVarP(&logsCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.StringVar(&logsCommand.Since, "since", "", "Show logs since TIMESTAMP")
	flags.Uint64Var(&logsCommand.Tail, "tail", 0, "Output the specified number of LINES at the end of the logs.  Defaults to 0, which prints all lines")
	flags.BoolVarP(&logsCommand.Timestamps, "timestamps", "t", false, "Output the timestamps in the log")
	flags.MarkHidden("details")

	flags.SetInterspersed(false)

	markFlagHiddenForRemoteClient("latest", flags)
}

func logsCmd(c *cliconfig.LogsValues) error {
	var err error

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	sinceTime := time.Time{}
	if c.Flag("since").Changed {
		// parse time, error out if something is wrong
		since, err := util.ParseInputTime(c.Since)
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.Since)
		}
		sinceTime = since
	}

	opts := &libpod.LogOptions{
		Details:    c.Details,
		Follow:     c.Follow,
		Since:      sinceTime,
		Tail:       c.Tail,
		Timestamps: c.Timestamps,
	}

	return runtime.Log(c, opts)
}
