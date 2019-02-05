package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	pruneSystemDescription = `
	podman system prune

        Remove unused data
`
	pruneSystemFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "remove all unused data",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Do not prompt for confirmation",
		},
		cli.BoolFlag{
			Name:  "volumes",
			Usage: "Prune volumes",
		},
	}
	pruneSystemCommand = cli.Command{
		Name:         "prune",
		Usage:        "Remove unused data",
		Description:  pruneSystemDescription,
		Action:       pruneSystemCmd,
		OnUsageError: usageErrorHandler,
		Flags:        pruneSystemFlags,
	}
)

func pruneSystemCmd(c *cli.Context) error {

	// Prompt for confirmation if --force is not set
	if !c.Bool("force") {
		reader := bufio.NewReader(os.Stdin)
		volumeString := ""
		if c.Bool("volumes") {
			volumeString = `
        - all volumes not used by at least one container`
		}
		fmt.Printf(`
WARNING! This will remove:
        - all stopped containers%s
        - all dangling images
        - all build cache
Are you sure you want to continue? [y/N] `, volumeString)
		ans, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(ans)[0] != 'y' {
			return nil
		}
	}

	runtime, err := adapter.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()
	fmt.Println("Deleted Containers")
	lasterr := pruneContainers(runtime, ctx, shared.Parallelize("rm"), false)
	if c.Bool("volumes") {
		fmt.Println("Deleted Volumes")
		err := volumePrune(runtime, getContext())
		if err != nil {
			if lasterr != nil {
				logrus.Errorf("%q", lasterr)
			}
			lasterr = err
		}
	}

	// Call prune; if any cids are returned, print them and then
	// return err in case an error also came up
	pruneCids, err := runtime.PruneImages(c.Bool("all"))
	if len(pruneCids) > 0 {
		fmt.Println("Deleted Images")
		for _, cid := range pruneCids {
			fmt.Println(cid)
		}
	}
	if err != nil {
		if lasterr != nil {
			logrus.Errorf("%q", lasterr)
		}
		lasterr = err
	}
	return lasterr
}
