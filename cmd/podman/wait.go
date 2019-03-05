package main

import (
	"fmt"
	"reflect"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	waitCommand cliconfig.WaitValues

	waitDescription = `
	podman wait

	Block until one or more containers stop and then print their exit codes
`
	_waitCommand = &cobra.Command{
		Use:   "wait [flags] CONTAINER [CONTAINER...]",
		Short: "Block on one or more containers",
		Long:  waitDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			waitCommand.InputArgs = args
			waitCommand.GlobalFlags = MainGlobalOpts
			return waitCmd(&waitCommand)
		},
		Example: `podman wait --latest
  podman wait --interval 5000 ctrID
  podman wait ctrID1 ctrID2`,
	}
)

func init() {
	waitCommand.Command = _waitCommand
	waitCommand.SetUsageTemplate(UsageTemplate())
	flags := waitCommand.Flags()
	flags.UintVarP(&waitCommand.Interval, "interval", "i", 250, "Milliseconds to wait before polling for completion")
	flags.BoolVarP(&waitCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func waitCmd(c *cliconfig.WaitValues) error {
	args := c.InputArgs
	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide at least one container name or id")
	}

	if c.Interval == 0 {
		return errors.Errorf("interval must be greater then 0")
	}
	interval := time.Duration(c.Interval) * time.Millisecond

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime")
	}
	defer runtime.Shutdown(false)

	ok, failures, err := runtime.WaitOnContainers(getContext(), c, interval)
	if err != nil {
		return err
	}

	for _, id := range ok {
		fmt.Println(id)
	}

	if len(failures) > 0 {
		keys := reflect.ValueOf(failures).MapKeys()
		lastKey := keys[len(keys)-1].String()
		lastErr := failures[lastKey]
		delete(failures, lastKey)

		for _, err := range failures {
			outputError(err)
		}
		return lastErr
	}
	return nil
}
