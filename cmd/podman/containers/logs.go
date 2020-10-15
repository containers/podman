package containers

import (
	"os"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// logsOptionsWrapper wraps entities.LogsOptions and prevents leaking
// CLI-only fields into the API types.
type logsOptionsWrapper struct {
	entities.ContainerLogsOptions

	SinceRaw string
}

var (
	logsOptions     logsOptionsWrapper
	logsDescription = `Retrieves logs for one or more containers.

  This does not guarantee execution order when combined with podman run (i.e., your run may not have generated any logs at the time you execute podman logs).
`
	logsCommand = &cobra.Command{
		Use:   "logs [options] CONTAINER [CONTAINER...]",
		Short: "Fetch the logs of one or more containers",
		Long:  logsDescription,
		Args: func(cmd *cobra.Command, args []string) error {
			switch {
			case registry.IsRemote() && logsOptions.Latest:
				return errors.New(cmd.Name() + " does not support 'latest' when run remotely")
			case registry.IsRemote() && len(args) > 1:
				return errors.New(cmd.Name() + " does not support multiple containers when run remotely")
			case logsOptions.Latest && len(args) > 0:
				return errors.New("--latest and containers cannot be used together")
			case !logsOptions.Latest && len(args) < 1:
				return errors.New("specify at least one container name or ID to log")
			}
			return nil
		},
		RunE: logs,
		Example: `podman logs ctrID
  podman logs --names ctrID1 ctrID2
  podman logs --tail 2 mywebserver
  podman logs --follow=true --since 10m ctrID
  podman logs mywebserver mydbserver`,
	}

	containerLogsCommand = &cobra.Command{
		Use:   logsCommand.Use,
		Short: logsCommand.Short,
		Long:  logsCommand.Long,
		Args:  logsCommand.Args,
		RunE:  logsCommand.RunE,
		Example: `podman container logs ctrID
		podman container logs --names ctrID1 ctrID2
		podman container logs --tail 2 mywebserver
		podman container logs --follow=true --since 10m ctrID
		podman container logs mywebserver mydbserver`,
	}
)

func init() {
	// logs
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: logsCommand,
	})
	logsFlags(logsCommand.Flags())
	validate.AddLatestFlag(logsCommand, &logsOptions.Latest)

	// container logs
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerLogsCommand,
		Parent:  containerCmd,
	})
	logsFlags(containerLogsCommand.Flags())
	validate.AddLatestFlag(containerLogsCommand, &logsOptions.Latest)
}

func logsFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&logsOptions.Details, "details", false, "Show extra details provided to the logs")
	flags.BoolVarP(&logsOptions.Follow, "follow", "f", false, "Follow log output.  The default is false")
	flags.StringVar(&logsOptions.SinceRaw, "since", "", "Show logs since TIMESTAMP")
	flags.Int64Var(&logsOptions.Tail, "tail", -1, "Output the specified number of LINES at the end of the logs.  Defaults to -1, which prints all lines")
	flags.BoolVarP(&logsOptions.Timestamps, "timestamps", "t", false, "Output the timestamps in the log")
	flags.BoolVarP(&logsOptions.Names, "names", "n", false, "Output the container name in the log")
	flags.SetInterspersed(false)
	_ = flags.MarkHidden("details")
}

func logs(_ *cobra.Command, args []string) error {
	if logsOptions.SinceRaw != "" {
		// parse time, error out if something is wrong
		since, err := util.ParseInputTime(logsOptions.SinceRaw)
		if err != nil {
			return errors.Wrapf(err, "error parsing --since %q", logsOptions.SinceRaw)
		}
		logsOptions.Since = since
	}
	logsOptions.Writer = os.Stdout
	return registry.ContainerEngine().ContainerLogs(registry.GetContext(), args, logsOptions.ContainerLogsOptions)
}
