package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	cleanupFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Cleans up all containers",
		},
		LatestFlag,
	}
	cleanupDescription = `
   podman container cleanup

   Cleans up mount points and network stacks on one or more containers from the host. The container name or ID can be used. This command is used internally when running containers, but can also be used if container cleanup has failed when a container exits.
`
	cleanupCommand = cli.Command{
		Name:         "cleanup",
		Usage:        "Cleanup network and mountpoints of one or more containers",
		Description:  cleanupDescription,
		Flags:        sortFlags(cleanupFlags),
		Action:       cleanupCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func cleanupCmd(c *cli.Context) error {
	if err := validateFlags(c, cleanupFlags); err != nil {
		return err
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if err := checkAllAndLatest(c); err != nil {
		return err
	}

	cleanupContainers, lastError := getAllOrLatestContainers(c, runtime, -1, "all")

	ctx := getContext()

	for _, ctr := range cleanupContainers {
		if err = ctr.Cleanup(ctx); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to cleanup container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
